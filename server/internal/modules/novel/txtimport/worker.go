package txtimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/importtask"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/jobs"
)

const JobType = "txt_import"

type Worker struct {
	DB           *sql.DB
	Jobs         *jobs.Repository
	Store        FileStore
	Writer       importtask.ChapterWriter
	WorkerID     string
	Lease        time.Duration
	PollInterval time.Duration
}

func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	if w.DB == nil || w.Jobs == nil || w.Store == nil || w.Writer == nil {
		return false, errors.New("TXT import worker dependencies are required")
	}
	job, err := w.Jobs.Claim(ctx, jobs.ClaimOptions{WorkerID: w.WorkerID, Module: "novel", Types: []string{JobType}, LeaseDuration: w.Lease})
	if errors.Is(err, jobs.ErrNoJob) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var payload struct {
		TaskID string `json:"txtImportTaskId"`
	}
	if err = json.Unmarshal(job.Payload, &payload); err != nil {
		return true, w.fail(ctx, job, 0, err, false)
	}
	taskID, err := parseTaskID(payload.TaskID)
	if err != nil {
		return true, w.fail(ctx, job, 0, err, false)
	}
	task, err := w.load(ctx, taskID)
	if err != nil {
		return true, w.fail(ctx, job, taskID, err, true)
	}
	if _, err = w.DB.ExecContext(ctx, `UPDATE novel_txt_import_task SET status='running',start_time=COALESCE(start_time,now()),attempt_count=$2,updated_at=now() WHERE id=$1`, taskID, job.AttemptCount); err != nil {
		return true, w.fail(ctx, job, taskID, err, true)
	}
	heartbeat, err := w.Jobs.KeepAlive(ctx, job.ID, w.WorkerID, w.Lease)
	if err != nil {
		return true, err
	}
	defer heartbeat.Stop()
	workCtx := heartbeat.Context
	size, sizeErr := strconv.ParseInt(task.ObjectByteSize, 10, 64)
	if sizeErr != nil || size <= 0 {
		if leaseErr := heartbeat.Stop(); leaseErr != nil {
			return true, leaseErr
		}
		return true, w.failTask(ctx, job, task, errors.New("invalid TXT object size"), false)
	}
	data, err := w.Store.Get(workCtx, ObjectMeta{Key: task.ObjectKey, SHA256: task.ObjectSHA256, ByteSize: size})
	if err != nil {
		if leaseErr := heartbeat.Stop(); leaseErr != nil {
			return true, leaseErr
		}
		return true, w.failTask(ctx, job, task, err, true)
	}
	parsed := importtask.ParseTXTChapters(data, strings.TrimSuffix(task.OriginalFilename, filepath.Ext(task.OriginalFilename)))
	converted := importtask.Task{ID: task.ID, TargetBookID: task.TargetBookID, ImportMode: "txt"}
	imported, err := w.Writer.Import(workCtx, converted, parsed)
	if err != nil {
		if leaseErr := heartbeat.Stop(); leaseErr != nil {
			return true, leaseErr
		}
		return true, w.failTask(ctx, job, task, err, false)
	}
	empty := 0
	for _, chapter := range parsed {
		if strings.TrimSpace(chapter.Content) == "" {
			empty++
		}
	}
	quality, summary := "passed", fmt.Sprintf("文件 %d 字节，识别章节 %d 个", len(data), len(parsed))
	if len(parsed) == 0 {
		quality, summary = "warning", "未识别章节标记"
	}
	_, err = w.DB.ExecContext(workCtx, `UPDATE novel_txt_import_task SET status='succeeded',quality_status=$2,quality_summary=$3,total_chapter_count=$4,imported_chapter_count=$5,empty_chapter_count=$6,end_time=now(),updated_at=now() WHERE id=$1`, taskID, quality, summary, len(parsed), imported, empty)
	if err != nil {
		if leaseErr := heartbeat.Stop(); leaseErr != nil {
			return true, leaseErr
		}
		return true, w.failTask(ctx, job, task, err, true)
	}
	if err = heartbeat.Finalize(func() error {
		return w.Jobs.Complete(ctx, job.ID, w.WorkerID, json.RawMessage(`{}`))
	}); err != nil {
		return true, err
	}
	return true, nil
}

func (w *Worker) Run(ctx context.Context) error {
	if w.WorkerID == "" {
		return errors.New("TXT import worker ID is required")
	}
	if w.Lease <= 0 {
		w.Lease = 10 * time.Minute
	}
	if w.PollInterval <= 0 {
		w.PollInterval = time.Second
	}
	ticker := time.NewTicker(w.PollInterval)
	defer ticker.Stop()
	for {
		if _, err := w.Jobs.RecoverExpired(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		for {
			worked, err := w.RunOnce(ctx)
			if errors.Is(err, context.Canceled) {
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

func parseTaskID(raw string) (int64, error) {
	var id int64
	if _, err := fmt.Sscan(raw, &id); err != nil || id <= 0 {
		return 0, errors.New("invalid TXT import task id")
	}
	return id, nil
}

func (w *Worker) load(ctx context.Context, id int64) (Task, error) {
	var task Task
	err := w.DB.QueryRowContext(ctx, `SELECT id::text,COALESCE(target_book_id::text,''),original_filename,object_key,object_sha256,object_byte_size::text,status,quality_status,quality_summary,total_chapter_count::text,imported_chapter_count::text,empty_chapter_count::text,duplicate_chapter_count::text,fail_reason,operator_name,attempt_count::text,max_attempts::text,COALESCE(start_time::text,''),COALESCE(end_time::text,''),created_at::text,updated_at::text FROM novel_txt_import_task WHERE id=$1`, id).Scan(&task.ID, &task.TargetBookID, &task.OriginalFilename, &task.ObjectKey, &task.ObjectSHA256, &task.ObjectByteSize, &task.Status, &task.QualityStatus, &task.QualitySummary, &task.TotalChapterCount, &task.ImportedChapterCount, &task.EmptyChapterCount, &task.DuplicateChapterCount, &task.FailReason, &task.OperatorName, &task.AttemptCount, &task.MaxAttempts, &task.StartTime, &task.EndTime, &task.CreatedAt, &task.UpdatedAt)
	return task, err
}

func (w *Worker) fail(ctx context.Context, job jobs.Job, taskID int64, err error, retryable bool) error {
	if taskID > 0 {
		status := "failed"
		if retryable {
			status = "pending"
		}
		_, _ = w.DB.ExecContext(ctx, `UPDATE novel_txt_import_task SET status=$2,fail_reason=$3,end_time=CASE WHEN $2='failed' THEN now() ELSE NULL END,updated_at=now() WHERE id=$1`, taskID, status, err.Error())
	}
	_ = w.Jobs.Fail(ctx, job.ID, w.WorkerID, jobs.Failure{Code: "TXT_IMPORT_FAILED", Message: err.Error(), Retryable: retryable})
	return err
}

func (w *Worker) failTask(ctx context.Context, job jobs.Job, task Task, err error, retryable bool) error {
	return w.fail(ctx, job, mustID(task.ID), err, retryable)
}

func mustID(raw string) int64 { id, _ := parseTaskID(raw); return id }
