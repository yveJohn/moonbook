//go:build integration

package importtask

import (
	"context"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"strconv"
	"testing"
	"time"
)

func TestImportTaskQueueAndRetryLifecycle(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var sourceID, boardID, candidateID string
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_crawl_forum_source(source_name,base_url) VALUES('任务测试来源','https://task.example.test') RETURNING id::text`).Scan(&sourceID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_crawl_forum_board(source_id,source_name,board_name,board_url) VALUES($1,'任务测试来源','任务板块','https://task.example.test/forum') RETURNING id::text`, sourceID).Scan(&boardID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_crawl_thread_candidate(source_id,source_name,board_id,board_name,forum_thread_id,thread_title,thread_url) VALUES($1,'任务测试来源',$2,'任务板块','task-thread-1','待导入帖子','https://task.example.test/thread-1') RETURNING id::text`, sourceID, boardID).Scan(&candidateID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM platform_job_attempts WHERE job_id IN (SELECT id FROM platform_jobs WHERE module='novel' AND job_type='forum_import' AND payload->>'candidateId'=$1)`, candidateID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM platform_jobs WHERE module='novel' AND job_type='forum_import' AND payload->>'candidateId'=$1`, candidateID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM novel_crawl_import_task WHERE candidate_id=$1`, candidateID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM novel_crawl_thread_candidate WHERE id=$1`, candidateID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM novel_crawl_forum_board WHERE id=$1`, boardID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM novel_crawl_forum_source WHERE id=$1`, sourceID)
	})
	r := SQLRepository{DB: db}
	task, err := r.Create(ctx, CreateInput{CandidateID: candidateID, ImportMode: "create", MergeStrategy: "source_thread", OperatorName: "integration"})
	if err != nil || task.ID == "" || task.Status != "pending" {
		t.Fatalf("task=%+v err=%v", task, err)
	}
	var candidateStatus string
	if err := db.QueryRowContext(ctx, `SELECT status FROM novel_crawl_thread_candidate WHERE id=$1`, candidateID).Scan(&candidateStatus); err != nil || candidateStatus != "importing" {
		t.Fatalf("candidate status=%s err=%v", candidateStatus, err)
	}
	id, _ := strconv.ParseInt(task.ID, 10, 64)
	if err := r.Cancel(ctx, id); err != nil {
		t.Fatal(err)
	}
	var taskStatus, jobStatus string
	if err := db.QueryRowContext(ctx, `SELECT t.status,j.status FROM novel_crawl_import_task t JOIN platform_jobs j ON j.id=t.platform_job_id WHERE t.id=$1`, id).Scan(&taskStatus, &jobStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT status FROM novel_crawl_thread_candidate WHERE id=$1`, candidateID).Scan(&candidateStatus); err != nil {
		t.Fatal(err)
	}
	if taskStatus != "cancelled" || jobStatus != "cancelled" || candidateStatus != "pending" {
		t.Fatalf("cancel task=%s job=%s candidate=%s", taskStatus, jobStatus, candidateStatus)
	}
	if _, err := r.Retry(ctx, id, CreateInput{}); err != nil {
		t.Fatal(err)
	}
	var attemptCount, maxAttempts int
	if err := db.QueryRowContext(ctx, `SELECT t.status,j.status,j.attempt_count,j.max_attempts FROM novel_crawl_import_task t JOIN platform_jobs j ON j.id=t.platform_job_id WHERE t.id=$1`, id).Scan(&taskStatus, &jobStatus, &attemptCount, &maxAttempts); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT status FROM novel_crawl_thread_candidate WHERE id=$1`, candidateID).Scan(&candidateStatus); err != nil {
		t.Fatal(err)
	}
	if taskStatus != "pending" || jobStatus != "pending" || candidateStatus != "importing" || maxAttempts < attemptCount+3 {
		t.Fatalf("retry task=%s job=%s candidate=%s attempts=%d/%d", taskStatus, jobStatus, candidateStatus, attemptCount, maxAttempts)
	}
}
