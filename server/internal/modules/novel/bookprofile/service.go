package bookprofile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/jobs"
)

const moduleName = "novel_book_profile"
const jobType = "generate"

type Service struct {
	DB      *sql.DB
	Objects *objectstore.Service
	Jobs    *jobs.Repository
}

func NewService(db *sql.DB, objects *objectstore.Service) *Service {
	return &Service{DB: db, Objects: objects, Jobs: jobs.NewRepository(db)}
}

func invalid(message string) error {
	return apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, message)
}
func conflict(message string) error {
	return apperror.New(apperror.CodeConflict, http.StatusConflict, message)
}
func parseID(value, label string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return 0, invalid(label + " 无效")
	}
	return id, nil
}
func nullID(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	id, _ := strconv.ParseInt(value, 10, 64)
	return id
}

func (s *Service) GetConfig(ctx context.Context) (Config, error) {
	var c Config
	var fallback sql.NullInt64
	err := s.DB.QueryRowContext(ctx, `SELECT id,auto_scan_enabled,auto_apply_enabled,process_ai_refusal_enabled,ai_config_id,refusal_fallback_ai_config_id,system_prompt,temperature::float8,max_tokens,timeout_seconds,request_interval_ms,retry_count,scan_batch_size,max_input_chars FROM novel_book_profile_config LIMIT 1`).Scan(&c.ID, &c.AutoScanEnabled, &c.AutoApplyEnabled, &c.ProcessAIRefusalEnabled, &c.AIConfigID, &fallback, &c.SystemPrompt, &c.Temperature, &c.MaxTokens, &c.TimeoutSeconds, &c.RequestIntervalMS, &c.RetryCount, &c.ScanBatchSize, &c.MaxInputChars)
	if errors.Is(err, sql.ErrNoRows) {
		return Config{}, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "请先保存作品资料补全配置")
	}
	if fallback.Valid {
		c.RefusalFallbackAIConfigID = strconv.FormatInt(fallback.Int64, 10)
	}
	return c, err
}
func normalizeConfig(c Config) (Config, error) {
	c.SystemPrompt = strings.TrimSpace(c.SystemPrompt)
	if _, err := parseID(c.AIConfigID, "AI 配置 ID"); err != nil {
		return c, err
	}
	if c.RefusalFallbackAIConfigID != "" {
		if _, err := parseID(c.RefusalFallbackAIConfigID, "备用 AI 配置 ID"); err != nil {
			return c, err
		}
		if c.RefusalFallbackAIConfigID == c.AIConfigID {
			return c, invalid("备用 AI 配置不能与主配置相同")
		}
	}
	if c.SystemPrompt == "" {
		return c, invalid("系统提示词不能为空")
	}
	if c.Temperature < 0 || c.Temperature > 2 {
		return c, invalid("temperature 必须在 0 到 2 之间")
	}
	if c.MaxTokens <= 0 {
		c.MaxTokens = 2000
	}
	if c.TimeoutSeconds <= 0 {
		c.TimeoutSeconds = 120
	}
	if c.RetryCount < 0 {
		c.RetryCount = 1
	}
	if c.ScanBatchSize <= 0 {
		c.ScanBatchSize = 5
	}
	if c.MaxInputChars < 1000 {
		c.MaxInputChars = 60000
	}
	return c, nil
}
func (s *Service) SaveConfig(ctx context.Context, c Config) (Config, error) {
	c, err := normalizeConfig(c)
	if err != nil {
		return Config{}, err
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO novel_book_profile_config(auto_scan_enabled,auto_apply_enabled,process_ai_refusal_enabled,ai_config_id,refusal_fallback_ai_config_id,system_prompt,temperature,max_tokens,timeout_seconds,request_interval_ms,retry_count,scan_batch_size,max_input_chars) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) ON CONFLICT ((true)) DO UPDATE SET auto_scan_enabled=excluded.auto_scan_enabled,auto_apply_enabled=excluded.auto_apply_enabled,process_ai_refusal_enabled=excluded.process_ai_refusal_enabled,ai_config_id=excluded.ai_config_id,refusal_fallback_ai_config_id=excluded.refusal_fallback_ai_config_id,system_prompt=excluded.system_prompt,temperature=excluded.temperature,max_tokens=excluded.max_tokens,timeout_seconds=excluded.timeout_seconds,request_interval_ms=excluded.request_interval_ms,retry_count=excluded.retry_count,scan_batch_size=excluded.scan_batch_size,max_input_chars=excluded.max_input_chars,updated_at=now()`, c.AutoScanEnabled, c.AutoApplyEnabled, c.ProcessAIRefusalEnabled, c.AIConfigID, nullID(c.RefusalFallbackAIConfigID), c.SystemPrompt, c.Temperature, c.MaxTokens, c.TimeoutSeconds, c.RequestIntervalMS, c.RetryCount, c.ScanBatchSize, c.MaxInputChars)
	if err != nil {
		return Config{}, invalid("作品资料补全配置无效")
	}
	return s.GetConfig(ctx)
}

func scanSuggestion(scanner interface{ Scan(...any) error }, detail bool) (Suggestion, error) {
	var v Suggestion
	var input, original, suggested, review []byte
	var retrySource, retryTarget sql.NullInt64
	err := scanner.Scan(&v.ID, &v.BookID, &v.BookName, &v.Status, &v.TriggerType, &v.InputMode, &v.InputDigest, &input, &original, &suggested, &review, &v.RawResponse, &v.ReviewerName, &v.RejectReason, &v.ReviewedAt, &v.AppliedAt, &v.ErrorMessage, &v.FailureType, &retrySource, &retryTarget, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return v, err
	}
	_ = json.Unmarshal(original, &v.Original)
	if len(suggested) > 0 {
		v.Suggested = &SuggestedSnapshot{}
		_ = json.Unmarshal(suggested, v.Suggested)
	}
	if len(review) > 0 {
		v.Review = &BookSnapshot{}
		_ = json.Unmarshal(review, v.Review)
	}
	if detail {
		_ = json.Unmarshal(input, &v.InputSnapshot)
	} else {
		v.RawResponse = ""
	}
	if retrySource.Valid {
		v.RetrySourceID = strconv.FormatInt(retrySource.Int64, 10)
	}
	if retryTarget.Valid {
		v.RetryTargetID = strconv.FormatInt(retryTarget.Int64, 10)
	}
	return v, nil
}

const suggestionSelect = `SELECT s.id,s.book_id,b.book_name,s.status,s.trigger_type,s.input_mode,s.input_digest,s.input_snapshot,s.original_snapshot,s.suggested_snapshot,s.review_snapshot,s.raw_response,s.reviewer_name,s.reject_reason,s.reviewed_at,s.applied_at,s.error_message,s.failure_type,s.retry_source_id,s.retry_target_id,s.created_at,s.updated_at FROM novel_book_profile_suggestion s JOIN novel_books b ON b.id=s.book_id `
const suggestionListSelect = `SELECT s.id,s.book_id,b.book_name,s.status,s.trigger_type,s.input_mode,s.input_digest,'{}'::jsonb,s.original_snapshot,s.suggested_snapshot,s.review_snapshot,''::text,s.reviewer_name,s.reject_reason,s.reviewed_at,s.applied_at,s.error_message,s.failure_type,s.retry_source_id,s.retry_target_id,s.created_at,s.updated_at FROM novel_book_profile_suggestion s JOIN novel_books b ON b.id=s.book_id `

func (s *Service) Get(ctx context.Context, id int64, detail bool) (Suggestion, error) {
	v, err := scanSuggestion(s.DB.QueryRowContext(ctx, suggestionSelect+`WHERE s.id=$1`, id), detail)
	if errors.Is(err, sql.ErrNoRows) {
		return v, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "作品资料建议不存在")
	}
	return v, err
}
func (s *Service) List(ctx context.Context, f Filter) ([]Suggestion, int64, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}
	if f.PageSize > 100 {
		f.PageSize = 100
	}
	bookID := int64(0)
	if f.BookID != "" {
		var err error
		bookID, err = parseID(f.BookID, "作品 ID")
		if err != nil {
			return nil, 0, err
		}
	}
	args := []any{bookID, "%" + strings.TrimSpace(f.BookName) + "%", strings.TrimSpace(f.Status), strings.TrimSpace(f.TriggerType), strings.TrimSpace(f.SuggestedCategoryCode)}
	where := `WHERE ($1=0 OR s.book_id=$1) AND ($2='%%' OR b.book_name ILIKE $2) AND ($3='' OR s.status=$3) AND ($4='' OR s.trigger_type=$4) AND ($5='' OR s.suggested_snapshot->>'categoryCode'=$5)`
	var total int64
	if err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM novel_book_profile_suggestion s JOIN novel_books b ON b.id=s.book_id `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := s.DB.QueryContext(ctx, suggestionListSelect+where+` ORDER BY s.created_at DESC,s.id DESC LIMIT $6 OFFSET $7`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []Suggestion{}
	for rows.Next() {
		v, scanErr := scanSuggestion(rows, false)
		if scanErr != nil {
			return nil, 0, scanErr
		}
		items = append(items, v)
	}
	return items, total, rows.Err()
}

func (s *Service) Generate(ctx context.Context, input GenerateInput, trigger string) (Suggestion, error) {
	bookID, err := parseID(input.BookID, "作品 ID")
	if err != nil {
		return Suggestion{}, err
	}
	config, err := s.GetConfig(ctx)
	if err != nil {
		return Suggestion{}, err
	}
	mode, digest, snapshot, original, err := (inputBuilder{s.DB, s.Objects}).build(ctx, bookID, config.MaxInputChars)
	if err != nil {
		return Suggestion{}, err
	}
	originalJSON, _ := json.Marshal(original)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Suggestion{}, err
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO novel_book_profile_suggestion(book_id,status,trigger_type,input_mode,input_digest,input_snapshot,original_snapshot,retry_source_id) VALUES($1,'running',$2,$3,$4,$5,$6,$7) RETURNING id`, bookID, trigger, mode, digest, snapshot, originalJSON, nullID(input.SourceSuggestionID)).Scan(&id)
	if err != nil {
		return Suggestion{}, conflict("该作品已有生成中或待审核的建议")
	}
	if input.SourceSuggestionID != "" {
		sourceID, _ := parseID(input.SourceSuggestionID, "来源建议 ID")
		_, _ = tx.ExecContext(ctx, `UPDATE novel_book_profile_suggestion SET status='retried',retry_target_id=$2,updated_at=now() WHERE id=$1 AND status='failed'`, sourceID, id)
	}
	if err = tx.Commit(); err != nil {
		return Suggestion{}, err
	}
	payload, _ := json.Marshal(map[string]string{"suggestionId": strconv.FormatInt(id, 10)})
	if _, _, err = s.Jobs.Enqueue(ctx, jobs.EnqueueOptions{Module: moduleName, Type: jobType, IdempotencyKey: "suggestion:" + strconv.FormatInt(id, 10), Payload: payload, MaxAttempts: config.RetryCount + 1}); err != nil {
		_, _ = s.DB.ExecContext(ctx, `UPDATE novel_book_profile_suggestion SET status='failed',failure_type='enqueue',error_message=$2,updated_at=now() WHERE id=$1 AND status='running'`, id, truncate(err.Error(), 1000))
		return Suggestion{}, err
	}
	return s.Get(ctx, id, true)
}
func (s *Service) BatchRegenerate(ctx context.Context, input BatchInput) (BatchResult, error) {
	result := BatchResult{TotalCount: len(input.SuggestionIDs), Failures: []BatchFailure{}}
	for _, rawID := range input.SuggestionIDs {
		id, err := parseID(rawID, "建议 ID")
		if err == nil {
			var bookID, status string
			err = s.DB.QueryRowContext(ctx, `SELECT book_id::text,status FROM novel_book_profile_suggestion WHERE id=$1`, id).Scan(&bookID, &status)
			if err == nil && status != "failed" {
				err = invalid("仅失败建议可重新生成")
			}
			if err == nil {
				_, err = s.Generate(ctx, GenerateInput{BookID: bookID, Regenerate: true, SourceSuggestionID: rawID}, "regenerate")
			}
		}
		if err != nil {
			result.Failures = append(result.Failures, BatchFailure{SuggestionID: rawID, Message: err.Error()})
		} else {
			result.SuccessCount++
		}
	}
	result.FailureCount = len(result.Failures)
	return result, nil
}

func (s *Service) Reject(ctx context.Context, id int64, input ReviewInput) error {
	reviewer := strings.TrimSpace(input.ReviewerName)
	if reviewer == "" {
		reviewer = "管理员"
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE novel_book_profile_suggestion SET status='rejected',reviewer_name=$2,reject_reason=$3,reviewed_at=now(),updated_at=now() WHERE id=$1 AND status IN ('pending','failed')`, id, reviewer, strings.TrimSpace(input.RejectReason))
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return conflict("建议状态已变化，无法拒绝")
	}
	return nil
}
func (s *Service) Apply(ctx context.Context, id int64, input ReviewInput) error {
	return s.apply(ctx, id, input, false)
}
func (s *Service) apply(ctx context.Context, id int64, input ReviewInput, automatic bool) error {
	input.BookName = strings.TrimSpace(input.BookName)
	input.CategoryCode = strings.TrimSpace(input.CategoryCode)
	input.BookDesc = strings.TrimSpace(input.BookDesc)
	if input.BookName == "" || len([]rune(input.BookName)) > 100 {
		return invalid("作品名称长度必须为 1 到 100")
	}
	if len([]rune(input.BookDesc)) > 2000 {
		return invalid("作品简介不能超过 2000 字")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var bookID int64
	var status string
	var originalJSON []byte
	if err = tx.QueryRowContext(ctx, `SELECT book_id,status,original_snapshot FROM novel_book_profile_suggestion WHERE id=$1 FOR UPDATE`, id).Scan(&bookID, &status, &originalJSON); errors.Is(err, sql.ErrNoRows) {
		return apperror.New(apperror.CodeNotFound, http.StatusNotFound, "作品资料建议不存在")
	}
	if err != nil {
		return err
	}
	if status != "pending" && !(status == "failed" && !automatic) {
		return conflict("建议状态已变化，无法应用")
	}
	var original BookSnapshot
	_ = json.Unmarshal(originalJSON, &original)
	current, err := loadBookSnapshot(ctx, tx, bookID)
	if err != nil {
		return err
	}
	if !sameSnapshot(original, current) {
		return conflict("作品资料已在建议生成后发生变化，请重新生成建议")
	}
	var primaryID int64
	var primaryName string
	if err = tx.QueryRowContext(ctx, `SELECT id,name FROM novel_categories WHERE code=$1 AND kind='primary' AND enabled AND deleted_at IS NULL`, input.CategoryCode).Scan(&primaryID, &primaryName); errors.Is(err, sql.ErrNoRows) {
		return invalid("主分类不存在或已停用")
	}
	if err != nil {
		return err
	}
	subs := []SnapshotCategory{}
	seen := map[string]bool{}
	for index, code := range input.SubCategoryCodes {
		code = strings.TrimSpace(code)
		if code == "" || seen[code] {
			continue
		}
		seen[code] = true
		var c SnapshotCategory
		if err = tx.QueryRowContext(ctx, `SELECT code,name FROM novel_categories WHERE code=$1 AND kind='sub' AND enabled AND deleted_at IS NULL`, code).Scan(&c.Code, &c.Name); errors.Is(err, sql.ErrNoRows) {
			return invalid("副分类不存在或已停用: " + code)
		}
		if err != nil {
			return err
		}
		c.Sort = index
		subs = append(subs, c)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE novel_books SET book_name=$2,primary_category_id=$3,category_code=$4,category_name=$5,description=$6,updated_at=now() WHERE id=$1 AND deleted_at IS NULL`, bookID, input.BookName, primaryID, input.CategoryCode, primaryName, input.BookDesc); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM novel_book_sub_categories WHERE book_id=$1`, bookID); err != nil {
		return err
	}
	for _, c := range subs {
		var categoryID int64
		if err = tx.QueryRowContext(ctx, `SELECT id FROM novel_categories WHERE code=$1 AND kind='sub' AND deleted_at IS NULL`, c.Code).Scan(&categoryID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO novel_book_sub_categories(book_id,category_id,category_code,category_name,sort) VALUES($1,$2,$3,$4,$5)`, bookID, categoryID, c.Code, c.Name, c.Sort); err != nil {
			return err
		}
	}
	review := BookSnapshot{BookName: input.BookName, CategoryCode: input.CategoryCode, CategoryName: primaryName, BookDesc: input.BookDesc, SubCategories: subs}
	reviewJSON, _ := json.Marshal(review)
	reviewer := strings.TrimSpace(input.ReviewerName)
	if automatic {
		reviewer = "自动应用"
	} else if reviewer == "" {
		reviewer = "管理员"
	}
	if _, err = tx.ExecContext(ctx, `UPDATE novel_book_profile_suggestion SET status='applied',review_snapshot=$2,reviewer_name=$3,reviewed_at=now(),applied_at=now(),updated_at=now() WHERE id=$1`, id, reviewJSON, reviewer); err != nil {
		return err
	}
	return tx.Commit()
}
func loadBookSnapshot(ctx context.Context, tx *sql.Tx, bookID int64) (BookSnapshot, error) {
	var v BookSnapshot
	if err := tx.QueryRowContext(ctx, `SELECT book_name,category_code,category_name,description FROM novel_books WHERE id=$1 AND deleted_at IS NULL`, bookID).Scan(&v.BookName, &v.CategoryCode, &v.CategoryName, &v.BookDesc); err != nil {
		return v, err
	}
	v.SubCategories = []SnapshotCategory{}
	rows, err := tx.QueryContext(ctx, `SELECT category_code,category_name,sort FROM novel_book_sub_categories WHERE book_id=$1 ORDER BY sort,category_id`, bookID)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	for rows.Next() {
		var c SnapshotCategory
		if err = rows.Scan(&c.Code, &c.Name, &c.Sort); err != nil {
			return v, err
		}
		v.SubCategories = append(v.SubCategories, c)
	}
	return v, rows.Err()
}
func sameSnapshot(a, b BookSnapshot) bool {
	if a.BookName != b.BookName || a.CategoryCode != b.CategoryCode || a.CategoryName != b.CategoryName || a.BookDesc != b.BookDesc || len(a.SubCategories) != len(b.SubCategories) {
		return false
	}
	for i := range a.SubCategories {
		if a.SubCategories[i] != b.SubCategories[i] {
			return false
		}
	}
	return true
}

func (s *Service) EnsureAutoSuggestions(ctx context.Context) error {
	config, err := s.GetConfig(ctx)
	if err != nil || !config.AutoScanEnabled {
		return err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT b.id::text FROM novel_books b WHERE b.deleted_at IS NULL AND b.publish_status<>'deprecated' AND EXISTS(SELECT 1 FROM novel_chapter_clean_result r WHERE r.book_id=b.id AND r.active AND r.status='success') AND NOT EXISTS(SELECT 1 FROM novel_book_profile_suggestion s WHERE s.book_id=b.id AND s.status IN ('running','pending')) ORDER BY b.updated_at,b.id LIMIT $1`, config.ScanBatchSize)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		_, _ = s.Generate(ctx, GenerateInput{BookID: id}, "auto_scan")
	}
	return nil
}
