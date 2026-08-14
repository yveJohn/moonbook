package chaptersummary

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

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/jobs"
)

const pendingSummarySQL = `active AND status='success' AND btrim(chapter_summary)='' AND cleaned_object_id IS NOT NULL`

type Service struct {
	DB   *sql.DB
	Jobs *jobs.Repository
}

func NewService(db *sql.DB) *Service { return &Service{DB: db, Jobs: jobs.NewRepository(db)} }

func invalid(message string) error {
	return apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, message)
}

func parseID(raw, name string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, invalid(name + "必须是正整数字符串")
	}
	return id, nil
}

func nullableID(raw string) (any, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	return parseID(raw, "拒答备用 AI 配置 ID")
}

func (s *Service) GetConfig(ctx context.Context) (Config, error) {
	var config Config
	var id, aiID int64
	var fallback sql.NullInt64
	var createdAt, updatedAt time.Time
	err := s.DB.QueryRowContext(ctx, `SELECT id,enabled,ai_config_id,refusal_fallback_ai_config_id,system_prompt,temperature,max_tokens,max_input_chars,timeout_seconds,request_interval_ms,retry_count,batch_size,created_at,updated_at FROM novel_chapter_summary_config ORDER BY id LIMIT 1`).Scan(
		&id, &config.Enabled, &aiID, &fallback, &config.SystemPrompt, &config.Temperature, &config.MaxTokens, &config.MaxInputChars, &config.TimeoutSeconds, &config.RequestIntervalMS, &config.RetryCount, &config.BatchSize, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Config{SystemPrompt: "请概括本章核心剧情，只返回 chapter_summary 字段。", Temperature: .1, MaxTokens: 1000, MaxInputChars: 120000, TimeoutSeconds: 120, RetryCount: 1, BatchSize: 50}, nil
	}
	if err != nil {
		return config, err
	}
	config.ID = strconv.FormatInt(id, 10)
	config.AIConfigID = strconv.FormatInt(aiID, 10)
	if fallback.Valid {
		config.RefusalFallbackAIConfigID = strconv.FormatInt(fallback.Int64, 10)
	}
	config.CreatedAt = createdAt.Format(time.RFC3339Nano)
	config.UpdatedAt = updatedAt.Format(time.RFC3339Nano)
	return config, nil
}

func (s *Service) SaveConfig(ctx context.Context, input ConfigInput) (Config, error) {
	aiID, err := parseID(input.AIConfigID, "AI 配置 ID")
	if err != nil {
		return Config{}, err
	}
	fallback, err := nullableID(input.RefusalFallbackAIConfigID)
	if err != nil {
		return Config{}, err
	}
	if fallbackID, ok := fallback.(int64); ok && fallbackID == aiID {
		return Config{}, invalid("拒答备用 AI 配置不能与主 AI 配置相同")
	}
	input.SystemPrompt = strings.TrimSpace(input.SystemPrompt)
	if input.SystemPrompt == "" || len(input.SystemPrompt) > 20000 || input.Temperature < 0 || input.Temperature > 2 || input.MaxTokens < 0 || input.MaxTokens > 1000000 || input.MaxInputChars < 1 || input.MaxInputChars > 1000000 || input.TimeoutSeconds < 1 || input.TimeoutSeconds > 1800 || input.RequestIntervalMS < 0 || input.RequestIntervalMS > 600000 || input.RetryCount < 0 || input.RetryCount > 20 || input.BatchSize < 1 || input.BatchSize > 1000 {
		return Config{}, invalid("章节简介补全配置参数无效")
	}
	var id int64
	err = s.DB.QueryRowContext(ctx, `INSERT INTO novel_chapter_summary_config(enabled,ai_config_id,refusal_fallback_ai_config_id,system_prompt,temperature,max_tokens,max_input_chars,timeout_seconds,request_interval_ms,retry_count,batch_size) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT ((true)) DO NOTHING RETURNING id`, input.Enabled, aiID, fallback, input.SystemPrompt, input.Temperature, input.MaxTokens, input.MaxInputChars, input.TimeoutSeconds, input.RequestIntervalMS, input.RetryCount, input.BatchSize).Scan(&id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Config{}, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		_, err = s.DB.ExecContext(ctx, `UPDATE novel_chapter_summary_config SET enabled=$1,ai_config_id=$2,refusal_fallback_ai_config_id=$3,system_prompt=$4,temperature=$5,max_tokens=$6,max_input_chars=$7,timeout_seconds=$8,request_interval_ms=$9,retry_count=$10,batch_size=$11,updated_at=now() WHERE id=(SELECT id FROM novel_chapter_summary_config ORDER BY id LIMIT 1)`, input.Enabled, aiID, fallback, input.SystemPrompt, input.Temperature, input.MaxTokens, input.MaxInputChars, input.TimeoutSeconds, input.RequestIntervalMS, input.RetryCount, input.BatchSize)
	}
	if err != nil {
		return Config{}, err
	}
	config, err := s.GetConfig(ctx)
	if err == nil && config.Enabled {
		_, err = s.EnsureAutoTask(ctx)
	}
	return config, err
}

const taskColumns = `id,status,automatic,total_count,processed_count,success_count,fail_count,current_result_id,current_book_id,current_chapter_id,stop_requested,operator_name,error_summary,latest_error_message,latest_raw_response,raw_response_expires_at,started_at,finished_at,created_at,updated_at`

func scanTask(row interface{ Scan(...any) error }) (Task, error) {
	var task Task
	var id int64
	var resultID, bookID, chapterID sql.NullInt64
	err := row.Scan(&id, &task.Status, &task.Automatic, &task.TotalCount, &task.ProcessedCount, &task.SuccessCount, &task.FailCount, &resultID, &bookID, &chapterID, &task.StopRequested, &task.OperatorName, &task.ErrorSummary, &task.LatestErrorMessage, &task.LatestRawResponse, &task.RawResponseExpiresAt, &task.StartedAt, &task.FinishedAt, &task.CreatedAt, &task.UpdatedAt)
	task.ID = strconv.FormatInt(id, 10)
	if resultID.Valid {
		task.CurrentResultID = strconv.FormatInt(resultID.Int64, 10)
	}
	if bookID.Valid {
		task.CurrentBookID = strconv.FormatInt(bookID.Int64, 10)
	}
	if chapterID.Valid {
		task.CurrentChapterID = strconv.FormatInt(chapterID.Int64, 10)
	}
	return task, err
}

func (s *Service) GetTask(ctx context.Context, id int64, includeRaw bool) (Task, error) {
	task, err := scanTask(s.DB.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM novel_chapter_summary_task WHERE id=$1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return task, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "简介补全任务不存在")
	}
	if !includeRaw {
		task.LatestRawResponse = ""
	}
	return task, err
}

func (s *Service) ListTasks(ctx context.Context, status string, page, size int) ([]Task, int64, error) {
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
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM novel_chapter_summary_task WHERE ($1='' OR status=$1)`, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+taskColumns+` FROM novel_chapter_summary_task WHERE ($1='' OR status=$1) ORDER BY id DESC LIMIT $2 OFFSET $3`, status, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []Task{}
	for rows.Next() {
		task, scanErr := scanTask(rows)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		task.LatestRawResponse = ""
		items = append(items, task)
	}
	return items, total, rows.Err()
}

func (s *Service) PendingCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM novel_chapter_clean_result WHERE `+pendingSummarySQL).Scan(&count)
	return count, err
}

func (s *Service) Start(ctx context.Context, input StartInput) (Task, error) {
	return s.start(ctx, false, strings.TrimSpace(input.OperatorName))
}

func (s *Service) start(ctx context.Context, automatic bool, operator string) (Task, error) {
	config, err := s.GetConfig(ctx)
	if err != nil {
		return Task{}, err
	}
	if config.ID == "" {
		return Task{}, invalid("请先保存章节简介补全配置")
	}
	if automatic && !config.Enabled {
		return Task{}, nil
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('novel:chapter-summary:start',0))`); err != nil {
		return Task{}, err
	}
	var runningID sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT max(id) FROM novel_chapter_summary_task WHERE status='running'`).Scan(&runningID); err != nil {
		return Task{}, err
	}
	if runningID.Valid {
		if automatic {
			return s.GetTask(ctx, runningID.Int64, false)
		}
		return Task{}, apperror.New(apperror.CodeConflict, http.StatusConflict, "已有章节简介补全任务正在运行")
	}
	var pending int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM novel_chapter_clean_result WHERE `+pendingSummarySQL).Scan(&pending); err != nil {
		return Task{}, err
	}
	if pending == 0 {
		if automatic {
			return Task{}, nil
		}
		return Task{}, invalid("没有待补全简介的清洗结果")
	}
	if len(operator) > 64 {
		operator = operator[:64]
	}
	var taskID int64
	if err = tx.QueryRowContext(ctx, `INSERT INTO novel_chapter_summary_task(status,automatic,total_count,operator_name) VALUES('running',$1,$2,$3) RETURNING id`, automatic, pending, operator).Scan(&taskID); err != nil {
		return Task{}, err
	}
	if err = tx.Commit(); err != nil {
		return Task{}, err
	}
	payload, _ := json.Marshal(map[string]string{"taskId": strconv.FormatInt(taskID, 10)})
	if _, _, err = s.Jobs.Enqueue(ctx, jobs.EnqueueOptions{Module: "novel", Type: "chapter_summary", IdempotencyKey: fmt.Sprintf("chapter-summary:%d", taskID), Payload: payload, MaxAttempts: 3}); err != nil {
		_, _ = s.DB.ExecContext(ctx, `UPDATE novel_chapter_summary_task SET status='failed',error_summary=$2,latest_error_message=$2,finished_at=now(),updated_at=now() WHERE id=$1`, taskID, "任务入队失败: "+err.Error())
		return Task{}, err
	}
	return s.GetTask(ctx, taskID, false)
}

func (s *Service) EnsureAutoTask(ctx context.Context) (Task, error) {
	return s.start(ctx, true, "自动补全")
}

func (s *Service) Stop(ctx context.Context, id int64) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE novel_chapter_summary_task SET stop_requested=true,updated_at=now() WHERE id=$1 AND status='running'`, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return invalid("任务不在运行中")
	}
	return nil
}

func (s *Service) Resume(ctx context.Context, id int64) (Task, error) {
	task, err := s.GetTask(ctx, id, false)
	if err != nil {
		return task, err
	}
	if task.Status != "stopped" && task.Status != "failed" {
		return task, invalid("只有已停止或失败任务可以续跑")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return task, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('novel:chapter-summary:start',0))`); err != nil {
		return task, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE novel_chapter_summary_task SET status='running',stop_requested=false,error_summary='',latest_error_message='',finished_at=NULL,updated_at=now() WHERE id=$1 AND status IN ('stopped','failed') AND NOT EXISTS(SELECT 1 FROM novel_chapter_summary_task WHERE status='running')`, id)
	if err != nil {
		return task, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return task, apperror.New(apperror.CodeConflict, http.StatusConflict, "已有章节简介补全任务正在运行")
	}
	if err = tx.Commit(); err != nil {
		return task, err
	}
	payload, _ := json.Marshal(map[string]string{"taskId": task.ID})
	_, _, err = s.Jobs.Enqueue(ctx, jobs.EnqueueOptions{Module: "novel", Type: "chapter_summary", IdempotencyKey: fmt.Sprintf("chapter-summary:%d:resume:%d", id, time.Now().UnixNano()), Payload: payload, MaxAttempts: 3})
	if err != nil {
		_, _ = s.DB.ExecContext(ctx, `UPDATE novel_chapter_summary_task SET status='failed',error_summary=$2,latest_error_message=$2,finished_at=now(),updated_at=now() WHERE id=$1`, id, "任务入队失败: "+err.Error())
		return task, err
	}
	return s.GetTask(ctx, id, false)
}

func (s *Service) GetStatus(ctx context.Context) (Status, error) {
	config, err := s.GetConfig(ctx)
	if err != nil {
		return Status{}, err
	}
	pending, err := s.PendingCount(ctx)
	if err != nil {
		return Status{}, err
	}
	status := Status{Config: config, PendingCount: pending, Enabled: config.Enabled}
	latest, latestErr := scanTask(s.DB.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM novel_chapter_summary_task ORDER BY id DESC LIMIT 1`))
	if latestErr != nil && !errors.Is(latestErr, sql.ErrNoRows) {
		return status, latestErr
	}
	if latestErr == nil {
		latest.LatestRawResponse = ""
		status.LatestTask = &latest
		if latest.TotalCount > 0 {
			status.ProgressPercent = latest.ProcessedCount * 100 / latest.TotalCount
		}
	}
	running, runningErr := scanTask(s.DB.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM novel_chapter_summary_task WHERE status='running' ORDER BY id DESC LIMIT 1`))
	if runningErr != nil && !errors.Is(runningErr, sql.ErrNoRows) {
		return status, runningErr
	}
	if runningErr == nil {
		running.LatestRawResponse = ""
		status.RunningTask, status.Running = &running, true
		if running.TotalCount > 0 {
			status.ProgressPercent = running.ProcessedCount * 100 / running.TotalCount
		}
	}
	return status, nil
}
