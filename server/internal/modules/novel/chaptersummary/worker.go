package chaptersummary

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/aiconfig"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/jobs"
)

type Worker struct {
	DB                            *sql.DB
	Jobs                          *jobs.Repository
	AI                            *aiconfig.Service
	Objects                       *objectstore.Service
	Transport                     Summarizer
	WorkerID                      string
	Lease, PollInterval, AutoScan time.Duration
}

type summaryPayload struct {
	TaskID string `json:"taskId"`
}
type candidate struct{ ResultID, BookID, ChapterID, ObjectID int64 }

func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	job, err := w.Jobs.Claim(ctx, jobs.ClaimOptions{WorkerID: w.WorkerID, Module: "novel", Types: []string{"chapter_summary"}, LeaseDuration: w.Lease})
	if errors.Is(err, jobs.ErrNoJob) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var payload summaryPayload
	if err = json.Unmarshal(job.Payload, &payload); err != nil {
		return true, w.fail(ctx, job, 0, errors.New("invalid chapter summary payload"))
	}
	taskID, err := strconv.ParseInt(payload.TaskID, 10, 64)
	if err != nil || taskID <= 0 {
		return true, w.fail(ctx, job, 0, errors.New("invalid chapter summary task ID"))
	}
	workCtx, cancel := context.WithCancel(ctx)
	renewDone := make(chan error, 1)
	go func() {
		interval := w.Lease / 3
		if interval < time.Second {
			interval = time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-workCtx.Done():
				renewDone <- nil
				return
			case <-ticker.C:
				if renewErr := w.Jobs.Renew(workCtx, job.ID, w.WorkerID, w.Lease); renewErr != nil {
					renewDone <- renewErr
					cancel()
					return
				}
			}
		}
	}()
	err = w.execute(workCtx, taskID)
	cancel()
	if renewErr := <-renewDone; renewErr != nil {
		err = renewErr
	}
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return true, err
		}
		return true, w.fail(ctx, job, taskID, err)
	}
	return true, w.Jobs.Complete(ctx, job.ID, w.WorkerID, json.RawMessage(`{}`))
}

func (w *Worker) execute(ctx context.Context, taskID int64) error {
	var status string
	var stop bool
	if err := w.DB.QueryRowContext(ctx, `SELECT status,stop_requested FROM novel_chapter_summary_task WHERE id=$1`, taskID).Scan(&status, &stop); err != nil {
		return err
	}
	if status != "running" {
		return nil
	}
	if stop {
		_, err := w.DB.ExecContext(ctx, `UPDATE novel_chapter_summary_task SET status='stopped',finished_at=now(),updated_at=now() WHERE id=$1 AND status='running'`, taskID)
		return err
	}
	config, err := NewService(w.DB).GetConfig(ctx)
	if err != nil {
		return err
	}
	rows, err := w.DB.QueryContext(ctx, `SELECT id,book_id,chapter_id,cleaned_object_id FROM novel_chapter_clean_result WHERE `+pendingSummarySQL+` ORDER BY id LIMIT $1`, config.BatchSize)
	if err != nil {
		return err
	}
	items := []candidate{}
	for rows.Next() {
		var item candidate
		if err = rows.Scan(&item.ResultID, &item.BookID, &item.ChapterID, &item.ObjectID); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err = rows.Close(); err != nil {
		return err
	}
	transport := w.Transport
	if transport == nil {
		transport = AITransport{}
	}
	for _, item := range items {
		if err = w.DB.QueryRowContext(ctx, `SELECT stop_requested FROM novel_chapter_summary_task WHERE id=$1`, taskID).Scan(&stop); err != nil {
			return err
		}
		if stop {
			_, err = w.DB.ExecContext(ctx, `UPDATE novel_chapter_summary_task SET status='stopped',finished_at=now(),updated_at=now() WHERE id=$1 AND status='running'`, taskID)
			return err
		}
		content, _, readErr := w.Objects.ReadStored(ctx, item.ObjectID)
		if readErr != nil {
			if err = w.recordFailure(ctx, taskID, item, "读取清洗稿失败: "+readErr.Error(), ""); err != nil {
				return err
			}
			continue
		}
		primary, resolveErr := w.AI.ResolveEnabled(ctx, config.AIConfigID)
		if resolveErr != nil {
			return resolveErr
		}
		summary, raw, callErr := w.call(ctx, config, primary, string(content), transport)
		if callErr != nil {
			if err = w.recordFailure(ctx, taskID, item, callErr.Error(), raw); err != nil {
				return err
			}
		} else if err = w.recordSuccess(ctx, taskID, item, summary); err != nil {
			return err
		}
		if config.RequestIntervalMS > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(config.RequestIntervalMS) * time.Millisecond):
			}
		}
	}
	if err = w.DB.QueryRowContext(ctx, `SELECT stop_requested FROM novel_chapter_summary_task WHERE id=$1`, taskID).Scan(&stop); err != nil {
		return err
	}
	finalStatus := "completed"
	if stop {
		finalStatus = "stopped"
	}
	_, err = w.DB.ExecContext(ctx, `UPDATE novel_chapter_summary_task SET status=$2,finished_at=now(),updated_at=now() WHERE id=$1 AND status='running'`, taskID, finalStatus)
	return err
}

func (w *Worker) call(ctx context.Context, config Config, primary aiconfig.RuntimeConfig, content string, transport Summarizer) (string, string, error) {
	content = truncateRunes(content, config.MaxInputChars)
	runtime := primary
	var summary, raw string
	var err error
	for attempt := 0; attempt <= config.RetryCount; attempt++ {
		summary, raw, err = transport.Summarize(ctx, runtime, config.SystemPrompt, content, config.Temperature, config.MaxTokens, time.Duration(config.TimeoutSeconds)*time.Second)
		if err == nil {
			_ = w.AI.RecordCallSuccess(ctx, runtime.Snapshot())
			return truncateRunes(strings.TrimSpace(summary), 1000), raw, nil
		}
		_ = w.AI.RecordCallFailure(ctx, runtime.Snapshot())
		if errors.Is(err, ErrAIRefusal) {
			break
		}
		if next, resolveErr := w.AI.ResolveEnabled(ctx, config.AIConfigID); resolveErr == nil {
			runtime = next
		}
	}
	if errors.Is(err, ErrAIRefusal) && config.RefusalFallbackAIConfigID != "" {
		fallback, fallbackErr := w.AI.ResolveEnabled(ctx, config.RefusalFallbackAIConfigID)
		if fallbackErr != nil {
			return "", raw, fmt.Errorf("resolve refusal fallback AI: %w", fallbackErr)
		}
		fallbackSummary, fallbackRaw, fallbackErr := transport.Summarize(ctx, fallback, config.SystemPrompt, content, config.Temperature, config.MaxTokens, time.Duration(config.TimeoutSeconds)*time.Second)
		if fallbackErr == nil {
			_ = w.AI.RecordCallSuccess(ctx, fallback.Snapshot())
			return truncateRunes(strings.TrimSpace(fallbackSummary), 1000), fallbackRaw, nil
		}
		_ = w.AI.RecordCallFailure(ctx, fallback.Snapshot())
		return "", "主AI:\n" + raw + "\n备用AI:\n" + fallbackRaw, fmt.Errorf("拒答备用 AI 调用失败: %w", fallbackErr)
	}
	return "", raw, err
}

func truncateRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func (w *Worker) recordSuccess(ctx context.Context, taskID int64, item candidate, summary string) error {
	if summary == "" {
		return w.recordFailure(ctx, taskID, item, "chapter_summary 不能为空", "")
	}
	tx, err := w.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE novel_chapter_clean_result SET chapter_summary=$2,updated_at=now() WHERE id=$1 AND `+pendingSummarySQL, item.ResultID, summary)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		err = w.updateProgressTx(ctx, tx, taskID, item, false, "候选已被其他任务处理", "")
	} else {
		err = w.updateProgressTx(ctx, tx, taskID, item, true, "", "")
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (w *Worker) recordFailure(ctx context.Context, taskID int64, item candidate, message, raw string) error {
	message = truncateRunes(strings.TrimSpace(message), 1000)
	tx, err := w.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = w.updateProgressTx(ctx, tx, taskID, item, false, message, raw); err != nil {
		return err
	}
	return tx.Commit()
}

func (w *Worker) updateProgressTx(ctx context.Context, tx *sql.Tx, taskID int64, item candidate, success bool, message, raw string) error {
	if success {
		_, err := tx.ExecContext(ctx, `UPDATE novel_chapter_summary_task SET processed_count=processed_count+1,success_count=success_count+1,current_result_id=$2,current_book_id=$3,current_chapter_id=$4,updated_at=now() WHERE id=$1`, taskID, item.ResultID, item.BookID, item.ChapterID)
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE novel_chapter_summary_task SET processed_count=processed_count+1,fail_count=fail_count+1,current_result_id=$2,current_book_id=$3,current_chapter_id=$4,error_summary=$5,latest_error_message=$5,latest_raw_response=$6,raw_response_expires_at=CASE WHEN $6='' THEN raw_response_expires_at ELSE now()+interval '7 days' END,updated_at=now() WHERE id=$1`, taskID, item.ResultID, item.BookID, item.ChapterID, message, raw)
	return err
}

func (w *Worker) fail(ctx context.Context, job jobs.Job, taskID int64, cause error) error {
	if taskID > 0 {
		message := truncateRunes(cause.Error(), 1000)
		_, _ = w.DB.ExecContext(ctx, `UPDATE novel_chapter_summary_task SET status='failed',error_summary=$2,latest_error_message=$2,finished_at=now(),updated_at=now() WHERE id=$1 AND status='running'`, taskID, message)
	}
	_ = w.Jobs.Fail(ctx, job.ID, w.WorkerID, jobs.Failure{Code: "CHAPTER_SUMMARY_FAILED", Message: cause.Error(), Retryable: false})
	return cause
}

func (w *Worker) Run(ctx context.Context) error {
	if strings.TrimSpace(w.WorkerID) == "" {
		return errors.New("chapter summary worker ID is required")
	}
	if w.Lease <= 0 {
		w.Lease = 10 * time.Minute
	}
	if w.PollInterval <= 0 {
		w.PollInterval = time.Second
	}
	if w.AutoScan <= 0 {
		w.AutoScan = 5 * time.Minute
	}
	ticker := time.NewTicker(w.PollInterval)
	defer ticker.Stop()
	lastAuto := time.Time{}
	for {
		_, _ = w.DB.ExecContext(ctx, `UPDATE novel_chapter_summary_task SET latest_raw_response='',raw_response_expires_at=NULL,updated_at=now() WHERE raw_response_expires_at<now() AND latest_raw_response<>''`)
		_, _ = w.Jobs.RecoverExpired(ctx)
		if lastAuto.IsZero() || time.Since(lastAuto) >= w.AutoScan {
			_, _ = NewService(w.DB).EnsureAutoTask(ctx)
			lastAuto = time.Now()
		}
		for {
			worked, err := w.RunOnce(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				if !worked {
					return fmt.Errorf("chapter summary worker: %w", err)
				}
				continue
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
