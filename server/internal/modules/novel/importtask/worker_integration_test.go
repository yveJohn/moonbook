//go:build integration

package importtask

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/fetchlog"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/jobs"
)

type integrationExecutor struct{ task chan Task }

func (e integrationExecutor) Execute(_ context.Context, task Task) (Result, error) {
	e.task <- task
	return Result{TotalChapterCount: 0, QualityStatus: "warning", QualitySummary: "集成测试无章节", Fetches: []PageFetch{
		{URL: task.ThreadURL, PageNo: 1, HTTPStatus: 200, ResponseBytes: 128, Elapsed: 5 * time.Millisecond, Status: "succeeded", Message: "第 1 页抓取成功"},
		{URL: task.ThreadURL + "?page=2", PageNo: 2, HTTPStatus: 200, ResponseBytes: 96, Elapsed: 4 * time.Millisecond, Status: "succeeded", Message: "第 2 页抓取成功"},
	}}, nil
}

type retryableIntegrationExecutor struct{}

func (retryableIntegrationExecutor) Execute(_ context.Context, task Task) (Result, error) {
	return Result{Fetches: []PageFetch{{URL: task.ThreadURL, PageNo: 1, HTTPStatus: 503, Status: "failed", Message: "论坛返回 HTTP 503"}}}, RetryableError{Err: fmt.Errorf("fixture retryable failure")}
}

func TestWorkerRunOnceCompletesPersistedImportJob(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	suffix := time.Now().UnixNano()
	sourceName := fmt.Sprintf("worker测试来源-%d", suffix)
	boardName := fmt.Sprintf("worker测试板块-%d", suffix)
	threadID := fmt.Sprintf("worker-thread-%d", suffix)
	var sourceID, boardID, candidateID, retryCandidateID string
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_crawl_forum_source(source_name,base_url,request_interval_ms,user_agent,cookie_text) VALUES($1,'https://worker.example.test',1234,'Worker-Fixture/1.0','session=worker') RETURNING id::text`, sourceName).Scan(&sourceID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_crawl_forum_board(source_id,source_name,board_name,board_url) VALUES($1,$2,$3,'https://worker.example.test/forum') RETURNING id::text`, sourceID, sourceName, boardName).Scan(&boardID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_crawl_thread_candidate(source_id,source_name,board_id,board_name,forum_thread_id,thread_title,thread_url) VALUES($1,$2,$3,$4,$5,'worker测试帖子','https://worker.example.test/thread-1') RETURNING id::text`, sourceID, sourceName, boardID, boardName, threadID).Scan(&candidateID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		for _, id := range []string{candidateID, retryCandidateID} {
			if id == "" {
				continue
			}
			_, _ = db.ExecContext(cleanup, `DELETE FROM novel_crawl_fetch_log WHERE task_id IN (SELECT id FROM novel_crawl_import_task WHERE candidate_id=$1)`, id)
			_, _ = db.ExecContext(cleanup, `DELETE FROM platform_job_attempts WHERE job_id IN (SELECT id FROM platform_jobs WHERE module='novel' AND job_type='forum_import' AND payload->>'candidateId'=$1)`, id)
			_, _ = db.ExecContext(cleanup, `DELETE FROM platform_jobs WHERE module='novel' AND job_type='forum_import' AND payload->>'candidateId'=$1`, id)
			_, _ = db.ExecContext(cleanup, `DELETE FROM novel_crawl_import_task WHERE candidate_id=$1`, id)
			_, _ = db.ExecContext(cleanup, `DELETE FROM novel_crawl_thread_candidate WHERE id=$1`, id)
		}
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_crawl_forum_board WHERE id=$1`, boardID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_crawl_forum_source WHERE id=$1`, sourceID)
	})
	task, err := (SQLRepository{DB: db}).Create(ctx, CreateInput{CandidateID: candidateID, ImportMode: "create", MergeStrategy: "source_thread", OperatorName: "worker-integration"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE platform_jobs SET available_at='2000-01-01 00:00:00+00' WHERE module='novel' AND job_type='forum_import' AND payload->>'candidateId'=$1`, candidateID); err != nil {
		t.Fatal(err)
	}
	seenTask := make(chan Task, 1)
	worker := &Worker{DB: db, Jobs: jobs.NewRepository(db), Logs: fetchlog.SQLRepository{DB: db}, Executor: integrationExecutor{task: seenTask}, WorkerID: "worker-integration", Lease: time.Minute}
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
	loaded := <-seenTask
	if loaded.requestInterval != 1234*time.Millisecond || loaded.sourceUserAgent != "Worker-Fixture/1.0" || loaded.sourceCookie != "session=worker" {
		t.Fatalf("source request policy was not loaded: interval=%s userAgent=%q cookie=%q", loaded.requestInterval, loaded.sourceUserAgent, loaded.sourceCookie)
	}
	var lastPage, pageLogs int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(last_import_page_no,0) FROM novel_crawl_thread_candidate WHERE id=$1`, candidateID).Scan(&lastPage); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM novel_crawl_fetch_log WHERE task_id=$1 AND stage='thread' AND status='succeeded'`, task.ID).Scan(&pageLogs); err != nil {
		t.Fatal(err)
	}
	if lastPage != 2 || pageLogs != 2 {
		t.Fatalf("lastPage=%d pageLogs=%d", lastPage, pageLogs)
	}

	retryThreadID := fmt.Sprintf("worker-retry-thread-%d", suffix)
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_crawl_thread_candidate(source_id,source_name,board_id,board_name,forum_thread_id,thread_title,thread_url) VALUES($1,$2,$3,$4,$5,'worker重试帖子','https://worker.example.test/retry-thread') RETURNING id::text`, sourceID, sourceName, boardID, boardName, retryThreadID).Scan(&retryCandidateID); err != nil {
		t.Fatal(err)
	}
	retryTask, err := (SQLRepository{DB: db}).Create(ctx, CreateInput{CandidateID: retryCandidateID, ImportMode: "incremental", MergeStrategy: "source_thread", OperatorName: "worker-retry-integration"})
	if err != nil {
		t.Fatal(err)
	}
	worker.Executor = retryableIntegrationExecutor{}
	for attempt := 1; attempt <= 3; attempt++ {
		if _, err := db.ExecContext(ctx, `UPDATE platform_jobs SET available_at='2000-01-01 00:00:00+00' WHERE module='novel' AND job_type='forum_import' AND payload->>'candidateId'=$1`, retryCandidateID); err != nil {
			t.Fatal(err)
		}
		worked, err := worker.RunOnce(ctx)
		if err != nil || !worked {
			t.Fatalf("retry attempt %d worked=%v err=%v", attempt, worked, err)
		}
	}
	if err := db.QueryRowContext(ctx, `SELECT t.status,j.status,c.status,t.attempt_count FROM novel_crawl_import_task t JOIN platform_jobs j ON j.id=t.platform_job_id JOIN novel_crawl_thread_candidate c ON c.id=t.candidate_id WHERE t.id=$1`, retryTask.ID).Scan(&status, &jobStatus, &candidateStatus, &lastPage); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || jobStatus != "failed" || candidateStatus != "failed" || lastPage != 3 {
		t.Fatalf("retry exhausted task=%s job=%s candidate=%s attempts=%d", status, jobStatus, candidateStatus, lastPage)
	}
}
