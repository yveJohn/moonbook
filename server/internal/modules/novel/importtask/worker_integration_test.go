//go:build integration

package importtask

import (
	"context"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/fetchlog"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/jobs"
)

type integrationExecutor struct{}

func (integrationExecutor) Execute(context.Context, Task) (Result, error) {
	return Result{TotalChapterCount: 0, QualityStatus: "warning", QualitySummary: "集成测试无章节"}, nil
}

func TestWorkerRunOnceCompletesPersistedImportJob(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var sourceID, boardID, candidateID string
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_crawl_forum_source(source_name,base_url) VALUES('worker测试来源','https://worker.example.test') RETURNING id::text`).Scan(&sourceID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_crawl_forum_board(source_id,source_name,board_name,board_url) VALUES($1,'worker测试来源','worker测试板块','https://worker.example.test/forum') RETURNING id::text`, sourceID).Scan(&boardID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_crawl_thread_candidate(source_id,source_name,board_id,board_name,forum_thread_id,thread_title,thread_url) VALUES($1,'worker测试来源',$2,'worker测试板块','worker-thread-1','worker测试帖子','https://worker.example.test/thread-1') RETURNING id::text`, sourceID, boardID).Scan(&candidateID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = db.ExecContext(cleanup, `DELETE FROM platform_job_attempts WHERE job_id IN (SELECT id FROM platform_jobs WHERE module='novel' AND job_type='forum_import' AND payload->>'candidateId'=$1)`, candidateID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM platform_jobs WHERE module='novel' AND job_type='forum_import' AND payload->>'candidateId'=$1`, candidateID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_crawl_import_task WHERE candidate_id=$1`, candidateID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_crawl_thread_candidate WHERE id=$1`, candidateID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_crawl_forum_board WHERE id=$1`, boardID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_crawl_forum_source WHERE id=$1`, sourceID)
	})
	task, err := (SQLRepository{DB: db}).Create(ctx, CreateInput{CandidateID: candidateID, ImportMode: "create", MergeStrategy: "source_thread", OperatorName: "worker-integration"})
	if err != nil {
		t.Fatal(err)
	}
	worker := &Worker{DB: db, Jobs: jobs.NewRepository(db), Logs: fetchlog.SQLRepository{DB: db}, Executor: integrationExecutor{}, WorkerID: "worker-integration", Lease: time.Minute}
	worked, err := worker.RunOnce(ctx)
	if err != nil || !worked {
		t.Fatalf("worked=%v err=%v", worked, err)
	}
	var status, candidateStatus, jobStatus string
	if err := db.QueryRowContext(ctx, `SELECT status FROM novel_crawl_import_task WHERE id=$1`, task.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT status FROM novel_crawl_thread_candidate WHERE id=$1`, candidateID).Scan(&candidateStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT status FROM platform_jobs WHERE module='novel' AND job_type='forum_import' AND payload->>'candidateId'=$1`, candidateID).Scan(&jobStatus); err != nil {
		t.Fatal(err)
	}
	if status != "succeeded" || candidateStatus != "imported" || jobStatus != "succeeded" {
		t.Fatalf("task=%s candidate=%s job=%s", status, candidateStatus, jobStatus)
	}
}
