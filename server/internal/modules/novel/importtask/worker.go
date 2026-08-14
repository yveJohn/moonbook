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
}
type Executor interface {
	Execute(context.Context, Task) (Result, error)
}
type RetryableError struct{ Err error }

func (e RetryableError) Error() string { return e.Err.Error() }
func (e RetryableError) Unwrap() error { return e.Err }

type Worker struct {
	DB       *sql.DB
	Jobs     *jobs.Repository
	Logs     fetchlog.SQLRepository
	Executor Executor
	Writer   ChapterWriter
	WorkerID string
	Lease    time.Duration
}
type ChapterWriter interface {
	Import(context.Context, Task, []ParsedChapter) (int, error)
}

func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
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
	_ = w.Logs.Append(ctx, fetchlog.Entry{TaskID: task.ID, SourceID: task.SourceID, SourceName: task.SourceName, BoardID: task.BoardID, BoardName: task.BoardName, ThreadURL: task.ThreadURL, Stage: "import", Status: "started", Message: "任务开始执行"})
	result, execErr := w.Executor.Execute(ctx, task)
	if execErr != nil {
		return true, w.finishFailureTask(ctx, job, task, execErr)
	}
	if w.Writer != nil && len(result.Chapters) > 0 {
		imported, writeErr := w.Writer.Import(ctx, task, result.Chapters)
		if writeErr != nil {
			return true, w.finishFailureTask(ctx, job, task, writeErr)
		}
		result.ImportedChapterCount = imported
	}
	if _, err = w.DB.ExecContext(ctx, `UPDATE novel_crawl_import_task SET status='succeeded',quality_status=$2,quality_summary=$3,total_chapter_count=$4,imported_chapter_count=$5,empty_chapter_count=$6,duplicate_chapter_count=$7,end_time=now(),updated_at=now() WHERE id=$1`, taskID, result.QualityStatus, result.QualitySummary, result.TotalChapterCount, result.ImportedChapterCount, result.EmptyChapterCount, result.DuplicateChapterCount); err != nil {
		return true, w.finishFailureTask(ctx, job, task, err)
	}
	_, err = w.DB.ExecContext(ctx, `UPDATE novel_crawl_thread_candidate SET status='imported',updated_at=now() WHERE id=$1`, task.CandidateID)
	if err != nil {
		return true, w.finishFailureTask(ctx, job, task, err)
	}
	_ = w.Logs.Append(ctx, fetchlog.Entry{TaskID: task.ID, SourceID: task.SourceID, SourceName: task.SourceName, BoardID: task.BoardID, BoardName: task.BoardName, ThreadURL: task.ThreadURL, Stage: "import", Status: "succeeded", ItemCount: result.ImportedChapterCount, Message: "任务执行成功"})
	if err = w.Jobs.Complete(ctx, job.ID, w.WorkerID, json.RawMessage(`{}`)); err != nil {
		return true, fmt.Errorf("complete platform job: %w", err)
	}
	return true, nil
}
func (w *Worker) loadTask(ctx context.Context, id int64) (Task, error) {
	var v Task
	err := scan(w.DB.QueryRowContext(ctx, `SELECT `+returningCols+` FROM novel_crawl_import_task WHERE id=$1`, id), &v)
	return v, err
}
func (w *Worker) finishFailure(ctx context.Context, job jobs.Job, err error, retryable bool) error {
	if retryable {
		_ = w.Jobs.Fail(ctx, job.ID, w.WorkerID, jobs.Failure{Code: "WORKER_ERROR", Message: err.Error(), Retryable: true})
	} else {
		_ = w.Jobs.Fail(ctx, job.ID, w.WorkerID, jobs.Failure{Code: "INVALID_TASK", Message: err.Error(), Retryable: false})
	}
	return err
}
func (w *Worker) finishFailureTask(ctx context.Context, job jobs.Job, task Task, err error) error {
	retryable := false
	var re RetryableError
	if errors.As(err, &re) {
		retryable = true
	}
	status := "failed"
	if retryable {
		status = "pending"
	}
	_, _ = w.DB.ExecContext(ctx, `UPDATE novel_crawl_import_task SET status=$2,fail_reason=$3,end_time=CASE WHEN $2='failed' THEN now() ELSE NULL END,updated_at=now() WHERE id=$1`, task.ID, status, err.Error())
	_ = w.Logs.Append(ctx, fetchlog.Entry{TaskID: task.ID, SourceID: task.SourceID, SourceName: task.SourceName, BoardID: task.BoardID, BoardName: task.BoardName, ThreadURL: task.ThreadURL, Stage: "import", Status: "failed", Message: err.Error()})
	return w.finishFailure(ctx, job, err, retryable)
}
