package importtask

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/fetchlog"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/jobs"
	"time"
)

const JobModule = "novel"
const JobType = "forum_import"

type Result struct {
	TotalChapterCount, ImportedChapterCount, EmptyChapterCount, DuplicateChapterCount int
	QualityStatus, QualitySummary                                                     string
	Chapters                                                                          []ParsedChapter
	Fetches                                                                           []PageFetch
}
type PageFetch struct {
	URL                               string
	PageNo, HTTPStatus, ResponseBytes int
	Elapsed                           time.Duration
	Status, Message                   string
}
type Executor interface {
	Execute(context.Context, Task) (Result, error)
}
type RetryableError struct{ Err error }

func (e RetryableError) Error() string { return e.Err.Error() }
func (e RetryableError) Unwrap() error { return e.Err }

type Worker struct {
	DB           *sql.DB
	Jobs         *jobs.Repository
	Logs         fetchlog.SQLRepository
	Executor     Executor
	Writer       ChapterWriter
	WorkerID     string
	Lease        time.Duration
	PollInterval time.Duration
}
type ChapterWriter interface {
	Import(context.Context, Task, []ParsedChapter) (int, error)
}

func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	if w.Jobs == nil || w.DB == nil || w.Executor == nil {
		return false, errors.New("import worker dependencies are required")
	}
	job, err := w.Jobs.Claim(ctx, jobs.ClaimOptions{WorkerID: w.WorkerID, Module: JobModule, Types: []string{JobType}, LeaseDuration: w.Lease})
	if errors.Is(err, jobs.ErrNoJob) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var p struct {
		ImportTaskID string `json:"importTaskId"`
	}
	if err = json.Unmarshal(job.Payload, &p); err != nil {
		return true, w.finishFailure(ctx, job, fmt.Errorf("invalid job payload: %w", err), false)
	}
	var taskID int64
	if _, err = fmt.Sscan(p.ImportTaskID, &taskID); err != nil || taskID <= 0 {
		return true, w.finishFailure(ctx, job, errors.New("invalid import task id"), false)
	}
	task, err := w.loadTask(ctx, taskID)
	if err != nil {
		return true, w.finishFailure(ctx, job, fmt.Errorf("load import task: %w", err), false)
	}
	if _, err = w.DB.ExecContext(ctx, `UPDATE novel_crawl_import_task SET status='running',start_time=COALESCE(start_time,now()),attempt_count=$2,updated_at=now() WHERE id=$1`, taskID, job.AttemptCount); err != nil {
		return true, w.finishFailure(ctx, job, err, true)
	}
	heartbeat, err := w.Jobs.KeepAlive(ctx, job.ID, w.WorkerID, w.Lease)
	if err != nil {
		return true, err
	}
	defer heartbeat.Stop()
	leaseCtx := heartbeat.Context
	_ = w.Logs.Append(leaseCtx, fetchlog.Entry{TaskID: task.ID, SourceID: task.SourceID, SourceName: task.SourceName, BoardID: task.BoardID, BoardName: task.BoardName, ThreadURL: task.ThreadURL, Stage: "import", Status: "started", Message: "任务开始执行"})
	result, execErr := w.Executor.Execute(leaseCtx, task)
	w.appendFetchLogs(leaseCtx, task, result.Fetches)
	if execErr != nil {
		if leaseErr := heartbeat.Stop(); leaseErr != nil {
			return true, leaseErr
		}
		return true, w.finishFailureTask(ctx, job, task, execErr)
	}
	if leaseCtx.Err() != nil {
		if leaseErr := heartbeat.Stop(); leaseErr != nil {
			return true, leaseErr
		}
		return true, leaseCtx.Err()
	}
	var taskStatus string
	if err = w.DB.QueryRowContext(leaseCtx, `SELECT status FROM novel_crawl_import_task WHERE id=$1`, task.ID).Scan(&taskStatus); err != nil {
		if leaseErr := heartbeat.Stop(); leaseErr != nil {
			return true, leaseErr
		}
		return true, w.finishFailureTask(ctx, job, task, err)
	}
	if taskStatus == "cancelled" {
		_ = w.Logs.Append(ctx, fetchlog.Entry{TaskID: task.ID, SourceID: task.SourceID, SourceName: task.SourceName, BoardID: task.BoardID, BoardName: task.BoardName, ThreadURL: task.ThreadURL, Stage: "import", Status: "skipped", Message: "任务已取消，跳过章节写入"})
		return true, nil
	}
	if w.Writer != nil && len(result.Chapters) > 0 {
		imported, writeErr := w.Writer.Import(leaseCtx, task, result.Chapters)
		if writeErr != nil {
			if leaseErr := heartbeat.Stop(); leaseErr != nil {
				return true, leaseErr
			}
			return true, w.finishFailureTask(ctx, job, task, writeErr)
		}
		result.ImportedChapterCount = imported
	}
	if _, err = w.DB.ExecContext(leaseCtx, `UPDATE novel_crawl_import_task SET status='succeeded',quality_status=$2,quality_summary=$3,total_chapter_count=$4,imported_chapter_count=$5,empty_chapter_count=$6,duplicate_chapter_count=$7,end_time=now(),updated_at=now() WHERE id=$1`, taskID, result.QualityStatus, result.QualitySummary, result.TotalChapterCount, result.ImportedChapterCount, result.EmptyChapterCount, result.DuplicateChapterCount); err != nil {
		if leaseErr := heartbeat.Stop(); leaseErr != nil {
			return true, leaseErr
		}
		return true, w.finishFailureTask(ctx, job, task, err)
	}
	_, err = w.DB.ExecContext(leaseCtx, `UPDATE novel_crawl_thread_candidate SET status='imported',last_import_page_no=$2,last_follow_time=now(),follow_count=follow_count+CASE WHEN $3='incremental' THEN 1 ELSE 0 END,follow_fail_reason='',updated_at=now() WHERE id=$1`, task.CandidateID, len(result.Fetches), task.ImportMode)
	if err != nil {
		if leaseErr := heartbeat.Stop(); leaseErr != nil {
			return true, leaseErr
		}
		return true, w.finishFailureTask(ctx, job, task, err)
	}
	_ = w.Logs.Append(leaseCtx, fetchlog.Entry{TaskID: task.ID, SourceID: task.SourceID, SourceName: task.SourceName, BoardID: task.BoardID, BoardName: task.BoardName, ThreadURL: task.ThreadURL, Stage: "import", Status: "succeeded", ItemCount: result.ImportedChapterCount, Message: "任务执行成功"})
	if err = heartbeat.Finalize(func() error {
		return w.Jobs.Complete(ctx, job.ID, w.WorkerID, json.RawMessage(`{}`))
	}); err != nil {
		return true, fmt.Errorf("complete platform job: %w", err)
	}
	return true, nil
}

// Run continuously claims persisted import jobs until the worker context is
// cancelled. Expired leases are recovered before each polling cycle so an
// application restart can resume unfinished work without duplicating results.
func (w *Worker) Run(ctx context.Context) error {
	if w.DB == nil || w.Jobs == nil || w.Executor == nil {
		return errors.New("import worker dependencies are required")
	}
	interval := w.PollInterval
	if interval <= 0 {
		interval = time.Second
	}
	if w.Lease <= 0 {
		w.Lease = 10 * time.Minute
	}
	if w.WorkerID == "" {
		return errors.New("import worker ID is required")
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := w.Jobs.RecoverExpired(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		for {
			worked, err := w.RunOnce(ctx)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			if err != nil {
				if worked {
					continue
				}
				return err
			}
			if !worked {
				break
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
func (w *Worker) loadTask(ctx context.Context, id int64) (Task, error) {
	var v Task
	err := scan(w.DB.QueryRowContext(ctx, `SELECT `+returningCols+` FROM novel_crawl_import_task WHERE id=$1`, id), &v)
	if err != nil {
		return Task{}, err
	}
	var intervalMS int
	err = w.DB.QueryRowContext(ctx, `SELECT request_interval_ms,COALESCE(user_agent,''),COALESCE(cookie_text,'') FROM novel_crawl_forum_source WHERE id=$1::bigint`, v.SourceID).Scan(&intervalMS, &v.sourceUserAgent, &v.sourceCookie)
	v.requestInterval = time.Duration(intervalMS) * time.Millisecond
	return v, err
}

func (w *Worker) appendFetchLogs(ctx context.Context, task Task, fetches []PageFetch) {
	for _, page := range fetches {
		_ = w.Logs.Append(ctx, fetchlog.Entry{
			TaskID: task.ID, SourceID: task.SourceID, SourceName: task.SourceName,
			BoardID: task.BoardID, BoardName: task.BoardName, ThreadURL: task.ThreadURL,
			RequestURL: page.URL, Stage: "thread", Status: page.Status,
			HTTPStatus: page.HTTPStatus, ResponseBytes: page.ResponseBytes,
			ElapsedMs: int(page.Elapsed.Milliseconds()), Message: page.Message,
		})
	}
}
func (w *Worker) finishFailure(ctx context.Context, job jobs.Job, err error, retryable bool) error {
	var persistErr error
	if retryable {
		persistErr = w.Jobs.Fail(ctx, job.ID, w.WorkerID, jobs.Failure{Code: "WORKER_ERROR", Message: err.Error(), Retryable: true})
	} else {
		persistErr = w.Jobs.Fail(ctx, job.ID, w.WorkerID, jobs.Failure{Code: "INVALID_TASK", Message: err.Error(), Retryable: false})
	}
	if persistErr != nil {
		return fmt.Errorf("persist import job failure: %w", persistErr)
	}
	return nil
}
func (w *Worker) finishFailureTask(ctx context.Context, job jobs.Job, task Task, err error) error {
	retryable := false
	var re RetryableError
	if errors.As(err, &re) {
		retryable = true
	}
	terminal := !retryable || job.AttemptCount >= job.MaxAttempts
	status := "failed"
	if !terminal {
		status = "pending"
	}
	if _, updateErr := w.DB.ExecContext(ctx, `UPDATE novel_crawl_import_task SET status=$2::varchar,fail_reason=$3,end_time=CASE WHEN $2::varchar='failed' THEN now() ELSE NULL END,updated_at=now() WHERE id=$1`, task.ID, status, err.Error()); updateErr != nil {
		return updateErr
	}
	if _, updateErr := w.DB.ExecContext(ctx, `UPDATE novel_crawl_thread_candidate SET status=CASE WHEN $3 THEN 'failed' ELSE status END,follow_fail_reason=$2,last_follow_time=now(),updated_at=now() WHERE id=$1`, task.CandidateID, err.Error(), terminal); updateErr != nil {
		return updateErr
	}
	_ = w.Logs.Append(ctx, fetchlog.Entry{TaskID: task.ID, SourceID: task.SourceID, SourceName: task.SourceName, BoardID: task.BoardID, BoardName: task.BoardName, ThreadURL: task.ThreadURL, Stage: "import", Status: "failed", Message: err.Error()})
	return w.finishFailure(ctx, job, err, retryable)
}
