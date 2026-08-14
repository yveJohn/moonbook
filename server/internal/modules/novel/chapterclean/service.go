package chapterclean

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/jobs"
)

type Service struct {
	DB      *sql.DB
	Jobs    *jobs.Repository
	Objects *objectstore.Service
}

func NewService(db *sql.DB, objects *objectstore.Service) *Service {
	return &Service{DB: db, Jobs: jobs.NewRepository(db), Objects: objects}
}
func bad(message string) error {
	return apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, message)
}
func parseLong(raw, name string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, bad(name + "必须是正整数字符串")
	}
	return id, nil
}
func nullableID(raw string) (any, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	return parseLong(raw, "备用 AI 配置 ID")
}

func (s *Service) GetConfig(ctx context.Context) (Config, error) {
	var c Config
	var id, ai int64
	var fallback sql.NullInt64
	var created, updated time.Time
	err := s.DB.QueryRowContext(ctx, `SELECT id,enabled,ai_config_id,refusal_fallback_ai_config_id,system_prompt,temperature,max_tokens,min_chapter_word_count,min_cleaned_text_percent,auto_success_word_count,timeout_seconds,request_interval_ms,retry_count,continue_on_failure,created_at,updated_at FROM novel_chapter_clean_config ORDER BY id LIMIT 1`).Scan(&id, &c.Enabled, &ai, &fallback, &c.SystemPrompt, &c.Temperature, &c.MaxTokens, &c.MinChapterWordCount, &c.MinCleanedTextPercent, &c.AutoSuccessWordCount, &c.TimeoutSeconds, &c.RequestIntervalMS, &c.RetryCount, &c.ContinueOnFailure, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Config{SystemPrompt: "清除广告、导航、站点提示等非小说内容，并返回 JSON。", Temperature: .1, MinCleanedTextPercent: 70, AutoSuccessWordCount: 3000, TimeoutSeconds: 120, RetryCount: 1}, nil
	}
	if err != nil {
		return c, err
	}
	c.ID = strconv.FormatInt(id, 10)
	c.AIConfigID = strconv.FormatInt(ai, 10)
	if fallback.Valid {
		c.RefusalFallbackAIConfigID = strconv.FormatInt(fallback.Int64, 10)
	}
	c.CreatedAt = created.Format(time.RFC3339Nano)
	c.UpdatedAt = updated.Format(time.RFC3339Nano)
	return c, nil
}
func (s *Service) SaveConfig(ctx context.Context, in ConfigInput) (Config, error) {
	ai, err := parseLong(in.AIConfigID, "AI 配置 ID")
	if err != nil {
		return Config{}, err
	}
	fallback, err := nullableID(in.RefusalFallbackAIConfigID)
	if err != nil {
		return Config{}, err
	}
	in.SystemPrompt = strings.TrimSpace(in.SystemPrompt)
	if in.SystemPrompt == "" || len(in.SystemPrompt) > 20000 || in.Temperature < 0 || in.Temperature > 2 || in.MinCleanedTextPercent < 0 || in.MinCleanedTextPercent > 100 || in.TimeoutSeconds < 1 || in.TimeoutSeconds > 1800 || in.RetryCount < 0 || in.RetryCount > 20 {
		return Config{}, bad("章节清洗配置参数无效")
	}
	var id int64
	err = s.DB.QueryRowContext(ctx, `INSERT INTO novel_chapter_clean_config(enabled,ai_config_id,refusal_fallback_ai_config_id,system_prompt,temperature,max_tokens,min_chapter_word_count,min_cleaned_text_percent,auto_success_word_count,timeout_seconds,request_interval_ms,retry_count,continue_on_failure) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) ON CONFLICT ((true)) DO NOTHING RETURNING id`, in.Enabled, ai, fallback, in.SystemPrompt, in.Temperature, in.MaxTokens, in.MinChapterWordCount, in.MinCleanedTextPercent, in.AutoSuccessWordCount, in.TimeoutSeconds, in.RequestIntervalMS, in.RetryCount, in.ContinueOnFailure).Scan(&id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Config{}, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		_, err = s.DB.ExecContext(ctx, `UPDATE novel_chapter_clean_config SET enabled=$1,ai_config_id=$2,refusal_fallback_ai_config_id=$3,system_prompt=$4,temperature=$5,max_tokens=$6,min_chapter_word_count=$7,min_cleaned_text_percent=$8,auto_success_word_count=$9,timeout_seconds=$10,request_interval_ms=$11,retry_count=$12,continue_on_failure=$13,updated_at=now() WHERE id=(SELECT id FROM novel_chapter_clean_config ORDER BY id LIMIT 1)`, in.Enabled, ai, fallback, in.SystemPrompt, in.Temperature, in.MaxTokens, in.MinChapterWordCount, in.MinCleanedTextPercent, in.AutoSuccessWordCount, in.TimeoutSeconds, in.RequestIntervalMS, in.RetryCount, in.ContinueOnFailure)
	}
	if err != nil {
		return Config{}, err
	}
	return s.GetConfig(ctx)
}

func scanTask(row interface{ Scan(...any) error }) (Task, error) {
	var t Task
	var id, book int64
	err := row.Scan(&id, &book, &t.BookName, &t.Status, &t.ForceReclean, &t.TotalCount, &t.ProcessedCount, &t.SuccessCount, &t.DiscardCount, &t.FailCount, &t.SkipCount, &t.AllChaptersDiscarded, &t.StopRequested, &t.OperatorName, &t.ErrorSummary, &t.StartedAt, &t.FinishedAt, &t.CreatedAt, &t.UpdatedAt)
	t.ID = strconv.FormatInt(id, 10)
	t.BookID = strconv.FormatInt(book, 10)
	return t, err
}

const taskCols = `id,book_id,book_name,status,force_reclean,total_count,processed_count,success_count,discard_count,fail_count,skip_count,all_chapters_discarded,stop_requested,operator_name,error_summary,started_at,finished_at,created_at,updated_at`

func (s *Service) ListTasks(ctx context.Context, status, bookRaw string, page, size int) ([]Task, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	var book any
	if strings.TrimSpace(bookRaw) != "" {
		v, e := parseLong(bookRaw, "书籍 ID")
		if e != nil {
			return nil, 0, e
		}
		book = v
	}
	var total int64
	err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM novel_chapter_clean_task WHERE ($1='' OR status=$1) AND ($2::bigint IS NULL OR book_id=$2)`, status, book).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+taskCols+` FROM novel_chapter_clean_task WHERE ($1='' OR status=$1) AND ($2::bigint IS NULL OR book_id=$2) ORDER BY id DESC LIMIT $3 OFFSET $4`, status, book, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		v, e := scanTask(rows)
		if e != nil {
			return nil, 0, e
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}
func (s *Service) GetTask(ctx context.Context, id int64) (Task, error) {
	t, err := scanTask(s.DB.QueryRowContext(ctx, `SELECT `+taskCols+` FROM novel_chapter_clean_task WHERE id=$1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return t, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "清洗任务不存在")
	}
	return t, err
}
func (s *Service) Start(ctx context.Context, in StartInput) (Task, error) {
	book, err := parseLong(in.BookID, "书籍 ID")
	if err != nil {
		return Task{}, err
	}
	cfg, err := s.GetConfig(ctx)
	if err != nil || cfg.ID == "" || !cfg.Enabled {
		if err == nil {
			err = bad("章节清洗尚未启用")
		}
		return Task{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('novel:chapter-clean:start',0))`); err != nil {
		return Task{}, err
	}
	var name string
	if err = tx.QueryRowContext(ctx, `SELECT book_name FROM novel_books WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, book).Scan(&name); errors.Is(err, sql.ErrNoRows) {
		return Task{}, bad("书籍不存在")
	} else if err != nil {
		return Task{}, err
	}
	var running bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_chapter_clean_task WHERE status='running')`).Scan(&running); err != nil {
		return Task{}, err
	}
	if running {
		return Task{}, apperror.New(apperror.CodeConflict, http.StatusConflict, "已有章节清洗任务正在运行")
	}
	statuses := []string{"pending", "failed", "expired"}
	if in.ForceReclean {
		statuses = []string{"pending", "cleaning", "cleaned", "discarded", "failed", "expired", "skipped"}
		_, err = tx.ExecContext(ctx, `UPDATE novel_chapter_clean_result SET active=false,status=CASE WHEN status IN ('failed','discarded') THEN status ELSE 'expired' END,updated_at=now() WHERE book_id=$1 AND active`, book)
		if err != nil {
			return Task{}, err
		}
	}
	var total int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM novel_chapters WHERE book_id=$1 AND deleted_at IS NULL AND ai_clean_status=ANY($2)`, book, statuses).Scan(&total); err != nil {
		return Task{}, err
	}
	if total == 0 {
		return Task{}, bad("没有可清洗章节")
	}
	var taskID int64
	operator := strings.TrimSpace(in.OperatorName)
	if err = tx.QueryRowContext(ctx, `INSERT INTO novel_chapter_clean_task(book_id,book_name,status,force_reclean,total_count,operator_name) VALUES($1,$2,'running',$3,$4,$5) RETURNING id`, book, name, in.ForceReclean, total, operator).Scan(&taskID); err != nil {
		return Task{}, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE novel_chapters SET ai_clean_status='cleaning',updated_at=now() WHERE book_id=$1 AND deleted_at IS NULL AND ai_clean_status=ANY($2)`, book, statuses)
	if err != nil {
		return Task{}, err
	}
	if err = tx.Commit(); err != nil {
		return Task{}, err
	}
	payload, _ := json.Marshal(map[string]string{"taskId": strconv.FormatInt(taskID, 10)})
	_, _, err = s.Jobs.Enqueue(ctx, jobs.EnqueueOptions{Module: "novel", Type: "chapter_clean", IdempotencyKey: fmt.Sprintf("chapter-clean:%d", taskID), Payload: payload, MaxAttempts: cfg.RetryCount + 1})
	if err != nil {
		_, _ = s.DB.ExecContext(ctx, `UPDATE novel_chapter_clean_task SET status='failed',error_summary='持久化任务入队失败',finished_at=now(),updated_at=now() WHERE id=$1`, taskID)
		_, _ = s.DB.ExecContext(ctx, `UPDATE novel_chapters SET ai_clean_status='pending',updated_at=now() WHERE book_id=$1 AND ai_clean_status='cleaning'`, book)
		return Task{}, err
	}
	return s.GetTask(ctx, taskID)
}
func (s *Service) Stop(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE novel_chapter_clean_task SET stop_requested=true,updated_at=now() WHERE id=$1 AND status='running'`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return bad("任务不在运行中")
	}
	return nil
}
func (s *Service) Resume(ctx context.Context, id int64) (Task, error) {
	t, err := s.GetTask(ctx, id)
	if err != nil {
		return t, err
	}
	if t.Status == "running" {
		return t, bad("任务已在运行")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return t, err
	}
	defer tx.Rollback()
	bookID, _ := strconv.ParseInt(t.BookID, 10, 64)
	if _, err = tx.ExecContext(ctx, `UPDATE novel_chapter_clean_result SET active=false,status='expired',updated_at=now() WHERE task_id=$1 AND active AND status='failed'`, id); err != nil {
		return t, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE novel_chapters SET ai_clean_status='cleaning',updated_at=now() WHERE book_id=$1 AND deleted_at IS NULL AND ai_clean_status IN ('pending','failed','expired')`, bookID); err != nil {
		return t, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE novel_chapter_clean_task SET status='running',stop_requested=false,error_summary='',finished_at=NULL,
		processed_count=(SELECT count(*) FROM novel_chapter_clean_result WHERE task_id=$1 AND active),
		success_count=(SELECT count(*) FROM novel_chapter_clean_result WHERE task_id=$1 AND active AND status='success'),
		discard_count=(SELECT count(*) FROM novel_chapter_clean_result WHERE task_id=$1 AND active AND status IN ('discarded','manual_discarded')),
		fail_count=0,skip_count=(SELECT count(*) FROM novel_chapter_clean_result WHERE task_id=$1 AND active AND status='skipped'),updated_at=now() WHERE id=$1`, id)
	if err != nil {
		return t, err
	}
	if err = tx.Commit(); err != nil {
		return t, err
	}
	payload, _ := json.Marshal(map[string]string{"taskId": t.ID})
	_, _, err = s.Jobs.Enqueue(ctx, jobs.EnqueueOptions{Module: "novel", Type: "chapter_clean", IdempotencyKey: fmt.Sprintf("chapter-clean:%d:%d", id, time.Now().UnixNano()), Payload: payload, MaxAttempts: 3})
	if err != nil {
		return t, err
	}
	return s.GetTask(ctx, id)
}

func scanResult(row interface{ Scan(...any) error }) (Result, error) {
	var r Result
	var id, task, book, chapter int64
	var cleaned, original sql.NullInt64
	err := row.Scan(&id, &task, &book, &chapter, &r.BookName, &r.ChapterName, &r.SourceChapterNo, &r.ContentType, &r.IsNovelBody, &r.CleanedChapterName, &cleaned, &original, &r.ChapterSummary, &r.CleanedWordCount, &r.RemovedNonNovel, &r.Confidence, &r.RawResponse, &r.Status, &r.ErrorMessage, &r.Active, &r.CreatedAt, &r.UpdatedAt)
	r.ID = strconv.FormatInt(id, 10)
	r.TaskID = strconv.FormatInt(task, 10)
	r.BookID = strconv.FormatInt(book, 10)
	r.ChapterID = strconv.FormatInt(chapter, 10)
	return r, err
}

const resultCols = `r.id,r.task_id,r.book_id,r.chapter_id,b.book_name,c.chapter_name,r.source_chapter_no,r.content_type,r.is_novel_body,r.cleaned_chapter_name,r.cleaned_object_id,r.original_object_id,r.chapter_summary,r.cleaned_word_count,r.removed_non_novel,r.confidence,r.raw_response,r.status,r.error_message,r.active,r.created_at,r.updated_at`

func (s *Service) ListResults(ctx context.Context, status string, page, size int) ([]Result, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	var total int64
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM novel_chapter_clean_result WHERE ($1='' OR status=$1)`, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+resultCols+` FROM novel_chapter_clean_result r JOIN novel_books b ON b.id=r.book_id JOIN novel_chapters c ON c.id=r.chapter_id WHERE ($1='' OR r.status=$1) ORDER BY r.id DESC LIMIT $2 OFFSET $3`, status, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Result{}
	for rows.Next() {
		r, e := scanResult(rows)
		if e != nil {
			return nil, 0, e
		}
		r.RawResponse = ""
		out = append(out, r)
	}
	return out, total, rows.Err()
}
func (s *Service) GetResult(ctx context.Context, id int64, content bool) (Result, error) {
	r, err := scanResult(s.DB.QueryRowContext(ctx, `SELECT `+resultCols+` FROM novel_chapter_clean_result r JOIN novel_books b ON b.id=r.book_id JOIN novel_chapters c ON c.id=r.chapter_id WHERE r.id=$1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return r, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "清洗结果不存在")
	}
	if err != nil || !content {
		return r, err
	}
	var cleaned, original sql.NullInt64
	if err = s.DB.QueryRowContext(ctx, `SELECT cleaned_object_id,original_object_id FROM novel_chapter_clean_result WHERE id=$1`, id).Scan(&cleaned, &original); err != nil {
		return r, err
	}
	if cleaned.Valid {
		data, _, e := s.Objects.ReadStored(ctx, cleaned.Int64)
		if e != nil {
			return r, e
		}
		r.CleanedText = string(data)
	}
	if original.Valid {
		data, _, e := s.Objects.ReadStored(ctx, original.Int64)
		if e != nil {
			return r, e
		}
		r.OriginalText = string(data)
	}
	return r, nil
}
func (s *Service) Review(ctx context.Context, id int64, in ReviewInput) error {
	if in.Status != "success" && in.Status != "manual_discarded" {
		return bad("审核状态无效")
	}
	r, err := s.GetResult(ctx, id, true)
	if err != nil {
		return err
	}
	if !r.Active {
		return bad("清洗结果已失效")
	}
	if in.Status == "manual_discarded" {
		tx, e := s.DB.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		defer tx.Rollback()
		if _, e = tx.ExecContext(ctx, `UPDATE novel_chapter_clean_result SET status='manual_discarded',updated_at=now() WHERE id=$1 AND active`, id); e == nil {
			_, e = tx.ExecContext(ctx, `UPDATE novel_chapters SET ai_clean_status='discarded',chapter_status='disabled',updated_at=now() WHERE id=$1`, r.ChapterID)
		}
		if e != nil {
			return e
		}
		return tx.Commit()
	}
	text := strings.TrimSpace(in.CleanedText)
	if text == "" {
		text = r.CleanedText
	}
	name := strings.TrimSpace(in.CleanedChapterName)
	if name == "" {
		name = r.CleanedChapterName
	}
	chapterID, _ := strconv.ParseInt(r.ChapterID, 10, 64)
	bookID, _ := strconv.ParseInt(r.BookID, 10, 64)
	obj, err := s.Objects.UploadVerified(ctx, objectstore.Target{Kind: objectstore.KindChapterContent, BookID: bookID, OwnerID: chapterID}, []byte(text), "text/plain; charset=utf-8")
	if err != nil {
		return err
	}
	return s.Objects.ActivateWithTx(ctx, obj.ID, func(tx *sql.Tx, _ objectstore.Target) error {
		_, err := tx.ExecContext(ctx, `UPDATE novel_chapters SET chapter_name=$2,word_count=$3,ai_clean_status='cleaned',updated_at=now() WHERE id=$1`, chapterID, name, countText(text))
		if err == nil {
			_, err = tx.ExecContext(ctx, `UPDATE novel_chapter_clean_result SET status='success',cleaned_chapter_name=$2,cleaned_word_count=$3,updated_at=now() WHERE id=$1`, id, name, countText(text))
		}
		return err
	})
}
func (s *Service) Reclean(ctx context.Context, id int64) error {
	r, err := s.GetResult(ctx, id, false)
	if err != nil {
		return err
	}
	if r.Status != "failed" && r.Status != "discarded" && r.Status != "manual_discarded" {
		return bad("该结果不可重新清洗")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE novel_chapter_clean_result SET active=false,status='expired',updated_at=now() WHERE id=$1`, id); err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE novel_chapters SET ai_clean_status='failed',updated_at=now() WHERE id=$1`, r.ChapterID)
	}
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	_, err = s.Start(ctx, StartInput{BookID: r.BookID, OperatorName: "结果重洗"})
	return err
}
func countText(s string) int {
	n := 0
	for _, r := range s {
		if !strings.ContainsRune(" \t\r\n", r) {
			n++
		}
	}
	return n
}
