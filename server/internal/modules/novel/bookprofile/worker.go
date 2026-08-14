package bookprofile

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
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/jobs"
)

type Worker struct {
	DB                            *sql.DB
	Jobs                          *jobs.Repository
	AI                            *aiconfig.Service
	Service                       *Service
	Transport                     Generator
	WorkerID                      string
	Lease, PollInterval, AutoScan time.Duration
}

func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	job, err := w.Jobs.Claim(ctx, jobs.ClaimOptions{WorkerID: w.WorkerID, Module: moduleName, Types: []string{jobType}, LeaseDuration: w.Lease})
	if errors.Is(err, jobs.ErrNoJob) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var payload struct {
		SuggestionID string `json:"suggestionId"`
	}
	if err = json.Unmarshal(job.Payload, &payload); err != nil {
		return true, w.fail(ctx, job, 0, err)
	}
	id, err := strconv.ParseInt(payload.SuggestionID, 10, 64)
	if err != nil {
		return true, w.fail(ctx, job, 0, err)
	}
	heartbeat, err := w.Jobs.KeepAlive(ctx, job.ID, w.WorkerID, w.Lease)
	if err != nil {
		return true, err
	}
	defer heartbeat.Stop()
	err = w.execute(heartbeat.Context, job, id)
	if err != nil {
		if renewErr := heartbeat.Stop(); renewErr != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
			return true, renewErr
		}
		return true, err
	}
	return true, heartbeat.Finalize(func() error {
		return w.Jobs.Complete(ctx, job.ID, w.WorkerID, json.RawMessage(`{"suggestionId":"`+strconv.FormatInt(id, 10)+`"}`))
	})
}
func (w *Worker) execute(ctx context.Context, job jobs.Job, id int64) error {
	config, err := w.Service.GetConfig(ctx)
	if err != nil {
		return w.fail(ctx, job, id, err)
	}
	suggestion, err := w.Service.Get(ctx, id, true)
	if err != nil {
		return w.fail(ctx, job, id, err)
	}
	if suggestion.Status != "running" {
		return nil
	}
	primary, err := w.AI.ResolveEnabled(ctx, config.AIConfigID)
	if err != nil {
		return w.fail(ctx, job, id, err)
	}
	transport := w.Transport
	if transport == nil {
		transport = AITransport{}
	}
	inputBytes, _ := json.Marshal(suggestion.InputSnapshot)
	var generated SuggestedSnapshot
	var raw string
	attempts := config.RetryCount + 1
	for attempt := 0; attempt < attempts; attempt++ {
		generated, raw, err = transport.Generate(ctx, primary, config.SystemPrompt, string(inputBytes), config.Temperature, config.MaxTokens, time.Duration(config.TimeoutSeconds)*time.Second)
		if err == nil {
			_ = w.AI.RecordCallSuccess(ctx, primary.Snapshot())
			break
		}
		_ = w.AI.RecordCallFailure(ctx, primary.Snapshot())
		if errors.Is(err, ErrAIRefusal) && config.ProcessAIRefusalEnabled && config.RefusalFallbackAIConfigID != "" {
			fallback, resolveErr := w.AI.ResolveEnabled(ctx, config.RefusalFallbackAIConfigID)
			if resolveErr == nil {
				var fallbackRaw string
				var fallbackErr error
				generated, fallbackRaw, fallbackErr = transport.Generate(ctx, fallback, config.SystemPrompt, string(inputBytes), config.Temperature, config.MaxTokens, time.Duration(config.TimeoutSeconds)*time.Second)
				raw = "主AI:\n" + raw + "\n备用AI:\n" + fallbackRaw
				err = fallbackErr
				if err == nil {
					_ = w.AI.RecordCallSuccess(ctx, fallback.Snapshot())
					break
				}
				_ = w.AI.RecordCallFailure(ctx, fallback.Snapshot())
			}
		}
		if config.RequestIntervalMS > 0 {
			select {
			case <-ctx.Done():
				return w.fail(ctx, job, id, ctx.Err())
			case <-time.After(time.Duration(config.RequestIntervalMS) * time.Millisecond):
			}
		}
	}
	if err != nil {
		return w.failRaw(ctx, job, id, err, raw)
	}
	if err = w.resolveCategories(ctx, &generated); err != nil {
		return w.failRaw(ctx, job, id, err, raw)
	}
	encoded, _ := json.Marshal(generated)
	result, err := w.DB.ExecContext(ctx, `UPDATE novel_book_profile_suggestion SET status='pending',suggested_snapshot=$2,raw_response=$3,raw_response_expires_at=now()+interval '7 days',error_message='',failure_type='',updated_at=now() WHERE id=$1 AND status='running'`, id, encoded, raw)
	if err != nil {
		return w.fail(ctx, job, id, err)
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return w.fail(ctx, job, id, errors.New("作品资料建议状态已变化"))
	}
	if config.AutoApplyEnabled {
		review := ReviewInput{BookName: generated.BookName, CategoryCode: generated.CategoryCode, BookDesc: generated.BookDesc}
		for _, c := range generated.SubCategories {
			review.SubCategoryCodes = append(review.SubCategoryCodes, c.Code)
		}
		if err = w.Service.apply(ctx, id, review, true); err != nil {
			return w.failRaw(ctx, job, id, fmt.Errorf("自动应用失败: %w", err), raw)
		}
	}
	return nil
}
func (w *Worker) resolveCategories(ctx context.Context, s *SuggestedSnapshot) error {
	if s.CategoryCode != "" {
		if err := w.DB.QueryRowContext(ctx, `SELECT name FROM novel_categories WHERE code=$1 AND kind='primary' AND enabled AND deleted_at IS NULL`, s.CategoryCode).Scan(&s.CategoryName); err == nil {
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		} else {
			s.CategoryCode = ""
		}
	}
	if s.CategoryCode == "" && s.CategoryName != "" {
		if err := w.DB.QueryRowContext(ctx, `SELECT code,name FROM novel_categories WHERE lower(name)=lower($1) AND kind='primary' AND enabled AND deleted_at IS NULL ORDER BY sort,id LIMIT 1`, s.CategoryName).Scan(&s.CategoryCode, &s.CategoryName); errors.Is(err, sql.ErrNoRows) {
			s.UnmatchedCategoryName = s.CategoryName
			return errors.New("AI 建议的主分类无法匹配")
		} else if err != nil {
			return err
		}
	}
	if s.CategoryCode == "" {
		return errors.New("AI 响应缺少有效主分类")
	}
	resolved := []SnapshotCategory{}
	for _, c := range s.SubCategories {
		var value SnapshotCategory
		err := sql.ErrNoRows
		if c.Code != "" {
			err = w.DB.QueryRowContext(ctx, `SELECT code,name,sort FROM novel_categories WHERE code=$1 AND kind='sub' AND enabled AND deleted_at IS NULL`, c.Code).Scan(&value.Code, &value.Name, &value.Sort)
		}
		if errors.Is(err, sql.ErrNoRows) && c.Name != "" {
			err = w.DB.QueryRowContext(ctx, `SELECT code,name,sort FROM novel_categories WHERE lower(name)=lower($1) AND kind='sub' AND enabled AND deleted_at IS NULL ORDER BY sort,id LIMIT 1`, c.Name).Scan(&value.Code, &value.Name, &value.Sort)
		}
		if err == nil {
			resolved = append(resolved, value)
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	s.SubCategories = resolved
	return nil
}
func (w *Worker) failRaw(ctx context.Context, job jobs.Job, id int64, cause error, raw string) error {
	message := truncate(cause.Error(), 1000)
	_, _ = w.DB.ExecContext(ctx, `UPDATE novel_book_profile_suggestion SET status='failed',error_message=$2,failure_type='generation',raw_response=$3,raw_response_expires_at=CASE WHEN $3='' THEN NULL ELSE now()+interval '7 days' END,updated_at=now() WHERE id=$1 AND status IN ('running','pending')`, id, message, raw)
	_ = w.Jobs.Fail(ctx, job.ID, w.WorkerID, jobs.Failure{Code: "BOOK_PROFILE_FAILED", Message: message, Retryable: false})
	return cause
}
func (w *Worker) fail(ctx context.Context, job jobs.Job, id int64, cause error) error {
	return w.failRaw(ctx, job, id, cause, "")
}
func truncate(value string, max int) string {
	r := []rune(strings.TrimSpace(value))
	if len(r) > max {
		r = r[:max]
	}
	return string(r)
}
func (w *Worker) Run(ctx context.Context) error {
	if strings.TrimSpace(w.WorkerID) == "" {
		return errors.New("book profile worker ID is required")
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
		_, _ = w.DB.ExecContext(ctx, `UPDATE novel_book_profile_suggestion SET raw_response='',raw_response_expires_at=NULL,updated_at=now() WHERE raw_response_expires_at<now() AND raw_response<>''`)
		_, _ = w.Jobs.RecoverExpired(ctx)
		if lastAuto.IsZero() || time.Since(lastAuto) >= w.AutoScan {
			_ = w.Service.EnsureAutoSuggestions(ctx)
			lastAuto = time.Now()
		}
		for {
			worked, err := w.RunOnce(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
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
