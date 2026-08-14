package chapterclean

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
	DB                  *sql.DB
	Jobs                *jobs.Repository
	AI                  *aiconfig.Service
	Objects             *objectstore.Service
	Transport           AITransport
	WorkerID            string
	Lease, PollInterval time.Duration
}
type cleanPayload struct {
	TaskID string `json:"taskId"`
}
type chapterWork struct {
	ID, BookID    int64
	No, WordCount int
	Name          string
}

func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	job, err := w.Jobs.Claim(ctx, jobs.ClaimOptions{WorkerID: w.WorkerID, Module: "novel", Types: []string{"chapter_clean"}, LeaseDuration: w.Lease})
	if errors.Is(err, jobs.ErrNoJob) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var payload cleanPayload
	if json.Unmarshal(job.Payload, &payload) != nil {
		return true, w.fail(ctx, job, errors.New("invalid chapter clean payload"), false)
	}
	taskID, err := strconv.ParseInt(payload.TaskID, 10, 64)
	if err != nil {
		return true, w.fail(ctx, job, err, false)
	}
	heartbeat, err := w.Jobs.KeepAlive(ctx, job.ID, w.WorkerID, w.Lease)
	if err != nil {
		return true, err
	}
	defer heartbeat.Stop()
	err = w.execute(heartbeat.Context, taskID)
	if err != nil {
		if renewErr := heartbeat.Stop(); renewErr != nil {
			return true, renewErr
		}
		return true, w.fail(ctx, job, err, true)
	}
	return true, heartbeat.Finalize(func() error {
		return w.Jobs.Complete(ctx, job.ID, w.WorkerID, json.RawMessage(`{}`))
	})
}
func (w *Worker) execute(ctx context.Context, taskID int64) error {
	var status string
	var stop bool
	var bookID int64
	if err := w.DB.QueryRowContext(ctx, `SELECT book_id,status,stop_requested FROM novel_chapter_clean_task WHERE id=$1`, taskID).Scan(&bookID, &status, &stop); err != nil {
		return err
	}
	if status != "running" {
		return nil
	}
	cfg, err := NewService(w.DB, w.Objects).GetConfig(ctx)
	if err != nil {
		return err
	}
	runtime, err := w.AI.ResolveEnabled(ctx, cfg.AIConfigID)
	if err != nil {
		return err
	}
	rows, err := w.DB.QueryContext(ctx, `SELECT id,book_id,chapter_no,chapter_name,word_count FROM novel_chapters WHERE book_id=$1 AND deleted_at IS NULL AND ai_clean_status='cleaning' ORDER BY chapter_no,id`, bookID)
	if err != nil {
		return err
	}
	items := []chapterWork{}
	for rows.Next() {
		var c chapterWork
		if err = rows.Scan(&c.ID, &c.BookID, &c.No, &c.Name, &c.WordCount); err != nil {
			rows.Close()
			return err
		}
		items = append(items, c)
	}
	rows.Close()
	consecutive := 0
	for _, chapter := range items {
		if err := w.DB.QueryRowContext(ctx, `SELECT stop_requested FROM novel_chapter_clean_task WHERE id=$1`, taskID).Scan(&stop); err != nil {
			return err
		}
		if stop {
			if _, err = w.DB.ExecContext(ctx, `UPDATE novel_chapter_clean_task SET status='stopped',finished_at=now(),updated_at=now() WHERE id=$1`, taskID); err == nil {
				_, err = w.DB.ExecContext(ctx, `UPDATE novel_chapters SET ai_clean_status='pending',updated_at=now() WHERE book_id=$1 AND ai_clean_status='cleaning'`, bookID)
			}
			return err
		}
		if cfg.MinChapterWordCount > 0 && chapter.WordCount < cfg.MinChapterWordCount {
			if err = w.recordSkip(ctx, taskID, chapter, "章节字数低于清洗阈值"); err != nil {
				return err
			}
			continue
		}
		data, original, err := w.Objects.ReadActive(ctx, objectstore.Target{Kind: objectstore.KindChapterContent, BookID: chapter.BookID, OwnerID: chapter.ID})
		if err != nil {
			if e := w.recordFailure(ctx, taskID, chapter, "读取章节正文失败: "+err.Error(), ""); e != nil {
				return e
			}
			continue
		}
		if cfg.AutoSuccessWordCount > 0 && chapter.WordCount >= cfg.AutoSuccessWordCount {
			err = w.recordResult(ctx, taskID, chapter, original.ID, AIResult{ContentType: "novel", IsNovelBody: true, CleanedChapterName: chapter.Name, CleanedText: string(data), Confidence: 1}, "")
			if err != nil {
				return err
			}
			continue
		}
		var aiResult AIResult
		var raw string
		for attempt := 0; attempt <= cfg.RetryCount; attempt++ {
			aiResult, raw, err = w.Transport.Call(ctx, runtime, cfg.SystemPrompt, chapter.Name, string(data), cfg.Temperature, cfg.MaxTokens, time.Duration(cfg.TimeoutSeconds)*time.Second)
			if err == nil {
				_ = w.AI.RecordCallSuccess(ctx, runtime.Snapshot())
				break
			}
			_ = w.AI.RecordCallFailure(ctx, runtime.Snapshot())
			if next, e := w.AI.ResolveEnabled(ctx, cfg.AIConfigID); e == nil {
				runtime = next
			}
		}
		if errors.Is(err, ErrAIRefusal) && cfg.RefusalFallbackAIConfigID != "" {
			if fallback, fallbackErr := w.AI.ResolveEnabled(ctx, cfg.RefusalFallbackAIConfigID); fallbackErr == nil {
				aiResult, raw, err = w.Transport.Call(ctx, fallback, cfg.SystemPrompt, chapter.Name, string(data), cfg.Temperature, cfg.MaxTokens, time.Duration(cfg.TimeoutSeconds)*time.Second)
				if err == nil {
					_ = w.AI.RecordCallSuccess(ctx, fallback.Snapshot())
				} else {
					_ = w.AI.RecordCallFailure(ctx, fallback.Snapshot())
				}
			}
		}
		if err != nil {
			consecutive++
			if e := w.recordFailure(ctx, taskID, chapter, err.Error(), raw); e != nil {
				return e
			}
			if !cfg.ContinueOnFailure && consecutive >= 3 {
				_, e := w.DB.ExecContext(ctx, `UPDATE novel_chapter_clean_task SET status='failed',error_summary='连续清洗失败3次，任务已自动停止',finished_at=now(),updated_at=now() WHERE id=$1`, taskID)
				if e == nil {
					_, e = w.DB.ExecContext(ctx, `UPDATE novel_chapters SET ai_clean_status='failed',updated_at=now() WHERE book_id=$1 AND ai_clean_status='cleaning'`, bookID)
				}
				return e
			}
			continue
		}
		consecutive = 0
		if aiResult.IsNovelBody && cfg.MinCleanedTextPercent > 0 && countText(aiResult.CleanedText)*100 < chapter.WordCount*cfg.MinCleanedTextPercent {
			if e := w.recordFailure(ctx, taskID, chapter, "清洗后正文低于最低保留比例", raw); e != nil {
				return e
			}
			continue
		}
		if err = w.recordResult(ctx, taskID, chapter, original.ID, aiResult, raw); err != nil {
			return err
		}
		if cfg.RequestIntervalMS > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(cfg.RequestIntervalMS) * time.Millisecond):
			}
		}
	}
	_, err = w.DB.ExecContext(ctx, `UPDATE novel_chapter_clean_task SET status='completed',all_chapters_discarded=(success_count=0 AND discard_count>0 AND fail_count=0),finished_at=now(),updated_at=now() WHERE id=$1 AND status='running'`, taskID)
	return err
}
func (w *Worker) recordSkip(ctx context.Context, taskID int64, c chapterWork, message string) error {
	return w.insertTerminal(ctx, taskID, c, "skipped", message, "", nil, nil)
}
func (w *Worker) recordFailure(ctx context.Context, taskID int64, c chapterWork, message, raw string) error {
	return w.insertTerminal(ctx, taskID, c, "failed", message, raw, nil, nil)
}
func (w *Worker) insertTerminal(ctx context.Context, taskID int64, c chapterWork, status, message, raw string, cleanedObject, originalObject any) error {
	tx, err := w.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `UPDATE novel_chapter_clean_result SET active=false,status='expired',updated_at=now() WHERE chapter_id=$1 AND active`, c.ID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO novel_chapter_clean_result(task_id,book_id,chapter_id,source_chapter_no,status,error_message,raw_response,raw_response_expires_at,cleaned_object_id,original_object_id) VALUES($1,$2,$3,$4,$5,$6,$7,now()+interval '7 days',$8,$9)`, taskID, c.BookID, c.ID, c.No, status, message, raw, cleanedObject, originalObject)
	if err != nil {
		return err
	}
	column := "fail_count"
	chapterStatus := "failed"
	if status == "skipped" {
		column = "skip_count"
		chapterStatus = "skipped"
	}
	_, err = tx.ExecContext(ctx, `UPDATE novel_chapter_clean_task SET processed_count=processed_count+1,`+column+`=`+column+`+1,updated_at=now() WHERE id=$1`, taskID)
	if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE novel_chapters SET ai_clean_status=$2,updated_at=now() WHERE id=$1`, c.ID, chapterStatus)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (w *Worker) recordResult(ctx context.Context, taskID int64, c chapterWork, originalID int64, result AIResult, raw string) error {
	var resultID int64
	tx, err := w.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `UPDATE novel_chapter_clean_result SET active=false,status='expired',updated_at=now() WHERE chapter_id=$1 AND active`, c.ID)
	if err == nil {
		err = tx.QueryRowContext(ctx, `INSERT INTO novel_chapter_clean_result(task_id,book_id,chapter_id,source_chapter_no,content_type,is_novel_body,cleaned_chapter_name,chapter_summary,cleaned_word_count,removed_non_novel,confidence,raw_response,raw_response_expires_at,status,original_object_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,now()+interval '7 days',$13,$14) RETURNING id`, taskID, c.BookID, c.ID, c.No, result.ContentType, result.IsNovelBody, result.CleanedChapterName, result.ChapterSummary, countText(result.CleanedText), result.RemovedNonNovel, result.Confidence, raw, map[bool]string{true: "success", false: "discarded"}[result.IsNovelBody], originalID).Scan(&resultID)
	}
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if result.IsNovelBody {
		artifact, e := w.Objects.UploadVerified(ctx, objectstore.Target{Kind: objectstore.KindChapterClean, BookID: c.BookID, OwnerID: resultID}, []byte(result.CleanedText), "text/plain; charset=utf-8")
		if e != nil {
			return e
		}
		live, e := w.Objects.UploadVerified(ctx, objectstore.Target{Kind: objectstore.KindChapterContent, BookID: c.BookID, OwnerID: c.ID}, []byte(result.CleanedText), "text/plain; charset=utf-8")
		if e != nil {
			return e
		}
		err = w.Objects.ActivateWithTx(ctx, live.ID, func(tx *sql.Tx, _ objectstore.Target) error {
			if _, e = tx.ExecContext(ctx, `UPDATE novel_chapter_clean_result SET cleaned_object_id=$2,updated_at=now() WHERE id=$1`, resultID, artifact.ID); e != nil {
				return e
			}
			_, e = tx.ExecContext(ctx, `UPDATE novel_chapters SET chapter_name=CASE WHEN $2='' THEN chapter_name ELSE $2 END,word_count=$3,ai_clean_status='cleaned',updated_at=now() WHERE id=$1`, c.ID, result.CleanedChapterName, countText(result.CleanedText))
			if e == nil {
				_, e = tx.ExecContext(ctx, `UPDATE novel_chapter_clean_task SET processed_count=processed_count+1,success_count=success_count+1,updated_at=now() WHERE id=$1`, taskID)
			}
			return e
		})
		if err != nil {
			_ = w.Objects.AbandonVerified(ctx, []int64{artifact.ID})
		}
		return err
	}
	if err != nil {
		return err
	}
	tx, err = w.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE novel_chapter_clean_task SET processed_count=processed_count+1,discard_count=discard_count+1,updated_at=now() WHERE id=$1`, taskID); err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE novel_chapters SET ai_clean_status='discarded',chapter_status='disabled',updated_at=now() WHERE id=$1`, c.ID)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (w *Worker) fail(ctx context.Context, job jobs.Job, cause error, retry bool) error {
	_ = w.Jobs.Fail(ctx, job.ID, w.WorkerID, jobs.Failure{Code: "CHAPTER_CLEAN_FAILED", Message: cause.Error(), Retryable: retry})
	return cause
}
func (w *Worker) Run(ctx context.Context) error {
	if w.WorkerID == "" {
		return errors.New("chapter clean worker ID is required")
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
		_, _ = w.DB.ExecContext(ctx, `UPDATE novel_chapter_clean_result SET raw_response='',raw_response_expires_at=NULL,updated_at=now() WHERE raw_response_expires_at<now() AND raw_response<>''`)
		_, _ = w.Jobs.RecoverExpired(ctx)
		for {
			worked, err := w.RunOnce(ctx)
			if err != nil && !strings.Contains(err.Error(), "context canceled") {
				if worked {
					continue
				}
				return fmt.Errorf("chapter clean worker: %w", err)
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
