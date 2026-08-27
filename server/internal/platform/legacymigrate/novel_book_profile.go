package legacymigrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type NovelBookProfileConfigStage struct{}

func (NovelBookProfileConfigStage) Name() string { return "novel-book-profile-config" }

type legacyProfileConfig struct {
	id, aiConfigID, fallbackID                                                  int64
	autoScan, autoApply                                                         int
	baseURL, requestMethod, model, prompt                                       string
	temperature                                                                 float64
	maxTokens, timeoutSeconds, intervalMS, retryCount, scanBatch, maxInputChars int
	created, updated                                                            sql.NullTime
}

func (NovelBookProfileConfigStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	if cursor != "" {
		return BatchResult{Done: true, Metadata: map[string]any{"skipped": "cursor_complete"}}, nil
	}
	ok, err := sourceTableExists(ctx, source, "novel_book_profile_config")
	if err != nil {
		return BatchResult{}, err
	}
	if !ok {
		return BatchResult{Done: true, Metadata: map[string]any{"skipped": "table_not_found"}}, nil
	}
	row := source.QueryRowContext(ctx, `SELECT id,COALESCE(auto_scan_enabled,0),COALESCE(auto_apply_enabled,0),COALESCE(ai_config_id,0),COALESCE(refusal_fallback_ai_config_id,0),COALESCE(base_url,''),COALESCE(request_method,'POST'),COALESCE(model,''),COALESCE(system_prompt,''),COALESCE(temperature,0.10),COALESCE(max_tokens,2000),COALESCE(timeout_seconds,120),COALESCE(request_interval_ms,0),COALESCE(retry_count,1),COALESCE(scan_batch_size,5),COALESCE(max_input_chars,60000),create_time,update_time FROM novel_book_profile_config ORDER BY id LIMIT 1`)
	var item legacyProfileConfig
	if err := row.Scan(&item.id, &item.autoScan, &item.autoApply, &item.aiConfigID, &item.fallbackID, &item.baseURL, &item.requestMethod, &item.model, &item.prompt, &item.temperature, &item.maxTokens, &item.timeoutSeconds, &item.intervalMS, &item.retryCount, &item.scanBatch, &item.maxInputChars, &item.created, &item.updated); err != nil {
		if err == sql.ErrNoRows {
			return BatchResult{Done: true, Metadata: map[string]any{"source": "novel_book_profile_config", "rows": 0}}, nil
		}
		return BatchResult{}, err
	}
	result := BatchResult{Processed: 1, NextCursor: "complete", Metadata: map[string]any{"source": "novel_book_profile_config", "apiKeyOmitted": true}}
	aiID, fallback, warning, err := resolveLegacyAIConfig(ctx, target, item.aiConfigID, item.fallbackID)
	if err != nil {
		return BatchResult{}, err
	}
	if warning != "" {
		result.Errors = append(result.Errors, mergeError("novel_book_profile_config", strconv.FormatInt(item.id, 10), "AI_CONFIG_FALLBACK_SELECTED", warning))
	}
	if item.id <= 0 || strings.TrimSpace(item.prompt) == "" || item.temperature < 0 || item.temperature > 2 || item.maxTokens < 1 || item.timeoutSeconds < 1 || item.intervalMS < 0 || item.retryCount < 0 || item.scanBatch < 1 || item.maxInputChars < 1000 {
		result.Errors = append(result.Errors, mergeError("novel_book_profile_config", strconv.FormatInt(item.id, 10), "INVALID_PROFILE_CONFIG", "legacy profile config fields are invalid"))
		return resultDone(result), nil
	}
	key := "moonbook-v1:novel_book_profile_config:" + strconv.FormatInt(item.id, 10)
	if err := checkLegacyKey(ctx, target, "novel_book_profile_config", item.id, key); err != nil {
		result.Errors = append(result.Errors, mergeError("novel_book_profile_config", strconv.FormatInt(item.id, 10), "SOURCE_KEY_CONFLICT", err.Error()))
		return resultDone(result), nil
	}
	_, err = target.ExecContext(ctx, `INSERT INTO novel_book_profile_config(id,auto_scan_enabled,auto_apply_enabled,process_ai_refusal_enabled,ai_config_id,refusal_fallback_ai_config_id,system_prompt,temperature,max_tokens,timeout_seconds,request_interval_ms,retry_count,scan_batch_size,max_input_chars,created_at,updated_at,legacy_source_key) VALUES($1,$2,$3,false,$4,NULLIF($5,0),$6,$7,$8,$9,$10,$11,$12,$13,COALESCE($14,now()),COALESCE($15,now()),$16) ON CONFLICT(id) DO UPDATE SET auto_scan_enabled=EXCLUDED.auto_scan_enabled,auto_apply_enabled=EXCLUDED.auto_apply_enabled,ai_config_id=EXCLUDED.ai_config_id,refusal_fallback_ai_config_id=EXCLUDED.refusal_fallback_ai_config_id,system_prompt=EXCLUDED.system_prompt,temperature=EXCLUDED.temperature,max_tokens=EXCLUDED.max_tokens,timeout_seconds=EXCLUDED.timeout_seconds,request_interval_ms=EXCLUDED.request_interval_ms,retry_count=EXCLUDED.retry_count,scan_batch_size=EXCLUDED.scan_batch_size,max_input_chars=EXCLUDED.max_input_chars,updated_at=EXCLUDED.updated_at,legacy_source_key=EXCLUDED.legacy_source_key`, item.id, item.autoScan != 0, item.autoApply != 0, aiID, fallback, strings.TrimSpace(item.prompt), item.temperature, item.maxTokens, item.timeoutSeconds, item.intervalMS, item.retryCount, item.scanBatch, item.maxInputChars, item.created, item.updated, key)
	if err != nil {
		return BatchResult{}, fmt.Errorf("upsert profile config: %w", err)
	}
	if _, err = target.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('novel_book_profile_config','id'),GREATEST(COALESCE((SELECT max(id) FROM novel_book_profile_config),1),1),true)`); err != nil {
		return BatchResult{}, err
	}
	return resultDone(result), nil
}

type NovelBookProfileSuggestionsStage struct{}

func (NovelBookProfileSuggestionsStage) Name() string { return "novel-book-profile-suggestions" }

type legacyProfileSuggestion struct {
	id, bookID, retrySource, retryTarget                                                                                                                                                                                                                                                                                                                                   int64
	status, trigger, inputMode, inputDigest                                                                                                                                                                                                                                                                                                                                string
	inputSnapshot, originalName, originalCategoryCode, originalCategoryName, originalDesc, originalSub, suggestedName, suggestedCategoryCode, suggestedCategoryName, suggestedUnmatched, suggestedReason, suggestedSub, suggestedSubReason, suggestedDesc, reviewName, reviewCategoryCode, reviewCategoryName, reviewDesc, reviewSub, reviewer, rejectReason, errorMessage string
	confidence                                                                                                                                                                                                                                                                                                                                                             sql.NullFloat64
	reviewed, applied, created, updated                                                                                                                                                                                                                                                                                                                                    sql.NullTime
}

func (NovelBookProfileSuggestionsStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	ok, err := sourceTableExists(ctx, source, "novel_book_profile_suggestion")
	if err != nil {
		return BatchResult{}, err
	}
	if !ok {
		return BatchResult{Done: true, Metadata: map[string]any{"skipped": "table_not_found"}}, nil
	}
	last, err := parseCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	rejectReason := "''"
	var hasRejectReason bool
	if err := source.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='novel_book_profile_suggestion' AND column_name='reject_reason')`).Scan(&hasRejectReason); err != nil {
		return BatchResult{}, err
	}
	if hasRejectReason {
		rejectReason = "reject_reason"
	}
	rows, err := source.QueryContext(ctx, fmt.Sprintf(`SELECT id,book_id,COALESCE(status,''),COALESCE(trigger_type,'manual'),COALESCE(input_mode,''),COALESCE(input_digest,''),COALESCE(original_book_name,''),COALESCE(original_category_code,''),COALESCE(original_category_name,''),COALESCE(original_book_desc,''),COALESCE(original_sub_categories_json,''),COALESCE(suggested_book_name,''),COALESCE(suggested_category_code,''),COALESCE(suggested_category_name,''),COALESCE(suggested_unmatched_category_name,''),COALESCE(category_reason,''),COALESCE(suggested_sub_categories_json,''),COALESCE(sub_category_reason,''),COALESCE(suggested_book_desc,''),confidence,COALESCE(review_book_name,''),COALESCE(review_category_code,''),COALESCE(review_category_name,''),COALESCE(review_book_desc,''),COALESCE(review_sub_categories_json,''),COALESCE(reviewer_name,''),COALESCE(%s,''),review_time,apply_time,COALESCE(error_message,''),COALESCE(retry_source_id,0),COALESCE(retry_target_id,0),create_time,update_time FROM novel_book_profile_suggestion WHERE id>? ORDER BY id LIMIT ?`, rejectReason), last, limit)
	if err != nil {
		return BatchResult{}, fmt.Errorf("query legacy profile suggestions: %w", err)
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": "novel_book_profile_suggestion", "rawResponseOmitted": true}}
	for rows.Next() {
		var item legacyProfileSuggestion
		if err := rows.Scan(&item.id, &item.bookID, &item.status, &item.trigger, &item.inputMode, &item.inputDigest, &item.originalName, &item.originalCategoryCode, &item.originalCategoryName, &item.originalDesc, &item.originalSub, &item.suggestedName, &item.suggestedCategoryCode, &item.suggestedCategoryName, &item.suggestedUnmatched, &item.suggestedReason, &item.suggestedSub, &item.suggestedSubReason, &item.suggestedDesc, &item.confidence, &item.reviewName, &item.reviewCategoryCode, &item.reviewCategoryName, &item.reviewDesc, &item.reviewSub, &item.reviewer, &item.rejectReason, &item.reviewed, &item.applied, &item.errorMessage, &item.retrySource, &item.retryTarget, &item.created, &item.updated); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(item.id, 10)
		key := "moonbook-v1:novel_book_profile_suggestion:" + result.NextCursor
		if item.id <= 0 || item.bookID <= 0 {
			result.Errors = append(result.Errors, mergeError("novel_book_profile_suggestion", result.NextCursor, "INVALID_PROFILE_SUGGESTION", "legacy profile suggestion identity is invalid"))
			continue
		}
		status, ok := mapLegacyProfileStatus(item.status)
		if !ok {
			result.Errors = append(result.Errors, mergeError("novel_book_profile_suggestion", result.NextCursor, "INVALID_PROFILE_STATUS", "legacy profile suggestion status is unsupported"))
			continue
		}
		trigger := strings.ToLower(strings.TrimSpace(item.trigger))
		if trigger != "auto_scan" && trigger != "manual" && trigger != "regenerate" {
			trigger = "manual"
			result.Errors = append(result.Errors, mergeError("novel_book_profile_suggestion", result.NextCursor, "INVALID_TRIGGER_TYPE", "legacy trigger type was mapped to manual"))
		}
		mode := strings.ToLower(strings.TrimSpace(item.inputMode))
		if mode != "chapter_summary" && mode != "chunk_summary" && mode != "sampled_cleaned_text" {
			mode = "sampled_cleaned_text"
		}
		original := profileJSON(map[string]any{"bookName": item.originalName, "categoryCode": item.originalCategoryCode, "categoryName": item.originalCategoryName, "bookDesc": item.originalDesc, "subCategories": jsonValue(item.originalSub)})
		suggested := profileOptionalJSON(map[string]any{"bookName": item.suggestedName, "categoryCode": item.suggestedCategoryCode, "categoryName": item.suggestedCategoryName, "unmatchedCategoryName": item.suggestedUnmatched, "categoryReason": item.suggestedReason, "subCategories": jsonValue(item.suggestedSub), "subCategoryReason": item.suggestedSubReason, "bookDesc": item.suggestedDesc})
		review := profileOptionalJSON(map[string]any{"bookName": item.reviewName, "categoryCode": item.reviewCategoryCode, "categoryName": item.reviewCategoryName, "bookDesc": item.reviewDesc, "subCategories": jsonValue(item.reviewSub)})
		digest := strings.ToLower(strings.TrimSpace(item.inputDigest))
		if len(digest) != 64 {
			sum := sha256.Sum256([]byte(original))
			digest = hex.EncodeToString(sum[:])
			result.Errors = append(result.Errors, mergeError("novel_book_profile_suggestion", result.NextCursor, "INPUT_DIGEST_RECALCULATED", "legacy input digest was missing or invalid"))
		}
		var bookOK bool
		if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_books WHERE id=$1)`, item.bookID).Scan(&bookOK); err != nil {
			return BatchResult{}, err
		}
		if !bookOK {
			result.Errors = append(result.Errors, mergeError("novel_book_profile_suggestion", result.NextCursor, "BOOK_NOT_FOUND", "profile suggestion references a missing book"))
			continue
		}
		if err := checkLegacyKey(ctx, target, "novel_book_profile_suggestion", item.id, key); err != nil {
			result.Errors = append(result.Errors, mergeError("novel_book_profile_suggestion", result.NextCursor, "SOURCE_KEY_CONFLICT", err.Error()))
			continue
		}
		if status == "pending" || status == "running" {
			var blocking bool
			if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_book_profile_suggestion WHERE book_id=$1 AND status IN ('running','pending') AND id<>$2)`, item.bookID, item.id).Scan(&blocking); err != nil {
				return BatchResult{}, err
			}
			if blocking {
				status = "expired"
				result.Errors = append(result.Errors, mergeError("novel_book_profile_suggestion", result.NextCursor, "BLOCKING_SUGGESTION_CONFLICT", "older pending suggestion was archived to satisfy target uniqueness"))
			}
		}
		var retrySource, retryTarget any
		if item.retrySource > 0 {
			retrySource = item.retrySource
		}
		if item.retryTarget > 0 {
			retryTarget = item.retryTarget
		}
		_, err = target.ExecContext(ctx, `INSERT INTO novel_book_profile_suggestion(id,book_id,status,trigger_type,input_mode,input_digest,input_snapshot,original_snapshot,suggested_snapshot,review_snapshot,raw_response,raw_response_expires_at,reviewer_name,reject_reason,reviewed_at,applied_at,error_message,failure_type,retry_source_id,retry_target_id,created_at,updated_at,legacy_source_key) VALUES($1,$2,$3,$4,$5,$6,'{}'::jsonb,$7,$8,$9,'',NULL,$10,$11,$12,$13,$14,'', $15,$16,COALESCE($17,now()),COALESCE($18,now()),$19) ON CONFLICT(id) DO UPDATE SET status=EXCLUDED.status,trigger_type=EXCLUDED.trigger_type,input_mode=EXCLUDED.input_mode,input_digest=EXCLUDED.input_digest,input_snapshot=EXCLUDED.input_snapshot,original_snapshot=EXCLUDED.original_snapshot,suggested_snapshot=EXCLUDED.suggested_snapshot,review_snapshot=EXCLUDED.review_snapshot,raw_response='',raw_response_expires_at=NULL,reviewer_name=EXCLUDED.reviewer_name,reject_reason=EXCLUDED.reject_reason,reviewed_at=EXCLUDED.reviewed_at,applied_at=EXCLUDED.applied_at,error_message=EXCLUDED.error_message,retry_source_id=EXCLUDED.retry_source_id,retry_target_id=EXCLUDED.retry_target_id,updated_at=EXCLUDED.updated_at,legacy_source_key=EXCLUDED.legacy_source_key`, item.id, item.bookID, status, trigger, mode, digest, original, suggested, review, item.reviewer, item.rejectReason, item.reviewed, item.applied, item.errorMessage, retrySource, retryTarget, item.created, item.updated, key)
		if err != nil {
			return BatchResult{}, fmt.Errorf("upsert profile suggestion %d: %w", item.id, err)
		}
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	if _, err = target.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('novel_book_profile_suggestion','id'),GREATEST(COALESCE((SELECT max(id) FROM novel_book_profile_suggestion),1),1),true)`); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

func mapLegacyProfileStatus(v string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "running", "pending", "approved", "rejected", "applied", "failed", "retried", "recovered", "expired":
		return strings.ToLower(strings.TrimSpace(v)), true
	case "success", "completed":
		return "pending", true
	default:
		return "", false
	}
}
func jsonValue(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []any{}
	}
	var value any
	if json.Unmarshal([]byte(raw), &value) == nil {
		return value
	}
	return []any{}
}
func profileJSON(value map[string]any) string { data, _ := json.Marshal(value); return string(data) }
func profileOptionalJSON(value map[string]any) any {
	for _, v := range value {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			data, _ := json.Marshal(value)
			return string(data)
		}
		if _, ok := v.([]any); ok {
			return profileJSON(value)
		}
	}
	return nil
}
