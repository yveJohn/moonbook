package legacymigrate

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

type NovelChapterSummaryConfigStage struct{}

func (NovelChapterSummaryConfigStage) Name() string { return "novel-chapter-summary-config" }

type legacySummaryConfig struct {
	id, aiConfigID, fallbackID                                                  int64
	enabled                                                                     int
	baseURL, requestMethod, model, prompt                                       string
	temperature                                                                 float64
	maxTokens, maxInputChars, timeoutSeconds, intervalMS, retryCount, batchSize int
	created, updated                                                            sql.NullTime
	apiKey                                                                      sql.NullString
}

func (NovelChapterSummaryConfigStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	if cursor != "" {
		return BatchResult{Done: true, Metadata: map[string]any{"skipped": "cursor_complete"}}, nil
	}
	ok, err := sourceTableExists(ctx, source, "novel_chapter_summary_config")
	if err != nil {
		return BatchResult{}, err
	}
	if !ok {
		return BatchResult{Done: true, Metadata: map[string]any{"skipped": "table_not_found"}}, nil
	}
	row := source.QueryRowContext(ctx, `SELECT id,COALESCE(enabled,0),COALESCE(ai_config_id,0),COALESCE(refusal_fallback_ai_config_id,0),COALESCE(base_url,''),COALESCE(request_method,'POST'),COALESCE(model,''),COALESCE(system_prompt,''),COALESCE(temperature,0.10),COALESCE(max_tokens,1000),COALESCE(max_input_chars,120000),COALESCE(timeout_seconds,120),COALESCE(request_interval_ms,0),COALESCE(retry_count,1),COALESCE(batch_size,50),api_key,create_time,update_time FROM novel_chapter_summary_config ORDER BY id LIMIT 1`)
	var item legacySummaryConfig
	if err := row.Scan(&item.id, &item.enabled, &item.aiConfigID, &item.fallbackID, &item.baseURL, &item.requestMethod, &item.model, &item.prompt, &item.temperature, &item.maxTokens, &item.maxInputChars, &item.timeoutSeconds, &item.intervalMS, &item.retryCount, &item.batchSize, &item.apiKey, &item.created, &item.updated); err != nil {
		if err == sql.ErrNoRows {
			return BatchResult{Done: true, Metadata: map[string]any{"source": "novel_chapter_summary_config", "rows": 0}}, nil
		}
		return BatchResult{}, err
	}
	result := BatchResult{Processed: 1, NextCursor: "complete", Metadata: map[string]any{"source": "novel_chapter_summary_config", "apiKeyOmitted": true}}
	aiID, fallback, warning, err := resolveLegacyAIConfig(ctx, target, item.aiConfigID, item.fallbackID)
	if err != nil {
		return BatchResult{}, err
	}
	if warning != "" {
		result.Errors = append(result.Errors, mergeError("novel_chapter_summary_config", strconv.FormatInt(item.id, 10), "AI_CONFIG_FALLBACK_SELECTED", warning))
	}
	if item.id <= 0 || strings.TrimSpace(item.prompt) == "" || item.temperature < 0 || item.temperature > 2 || item.maxTokens < 0 || item.maxInputChars < 1 || item.timeoutSeconds < 1 || item.intervalMS < 0 || item.retryCount < 0 || item.batchSize < 1 {
		result.Errors = append(result.Errors, mergeError("novel_chapter_summary_config", strconv.FormatInt(item.id, 10), "INVALID_SUMMARY_CONFIG", "legacy summary config fields are invalid"))
		return resultDone(result), nil
	}
	_, err = target.ExecContext(ctx, `INSERT INTO novel_chapter_summary_config(id,enabled,ai_config_id,refusal_fallback_ai_config_id,system_prompt,temperature,max_tokens,max_input_chars,timeout_seconds,request_interval_ms,retry_count,batch_size,created_at,updated_at) VALUES($1,$2,$3,NULLIF($4,0),$5,$6,$7,$8,$9,$10,$11,$12,COALESCE($13,now()),COALESCE($14,now())) ON CONFLICT(id) DO UPDATE SET enabled=EXCLUDED.enabled,ai_config_id=EXCLUDED.ai_config_id,refusal_fallback_ai_config_id=EXCLUDED.refusal_fallback_ai_config_id,system_prompt=EXCLUDED.system_prompt,temperature=EXCLUDED.temperature,max_tokens=EXCLUDED.max_tokens,max_input_chars=EXCLUDED.max_input_chars,timeout_seconds=EXCLUDED.timeout_seconds,request_interval_ms=EXCLUDED.request_interval_ms,retry_count=EXCLUDED.retry_count,batch_size=EXCLUDED.batch_size,updated_at=EXCLUDED.updated_at`, item.id, item.enabled != 0, aiID, fallback, strings.TrimSpace(item.prompt), item.temperature, item.maxTokens, item.maxInputChars, item.timeoutSeconds, item.intervalMS, item.retryCount, item.batchSize, item.created, item.updated)
	if err != nil {
		return BatchResult{}, fmt.Errorf("upsert summary config: %w", err)
	}
	if _, err = target.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('novel_chapter_summary_config','id'),GREATEST(COALESCE((SELECT max(id) FROM novel_chapter_summary_config),1),1),true)`); err != nil {
		return BatchResult{}, err
	}
	return resultDone(result), nil
}

type NovelChapterSummaryTaskStage struct{}

func (NovelChapterSummaryTaskStage) Name() string { return "novel-chapter-summary-tasks" }

type legacySummaryTask struct {
	id                                    int64
	status                                string
	stop, total, processed, success, fail int
	resultID, bookID, chapterID           sql.NullInt64
	errorSummary, latestError             string
	start, end, created, updated          sql.NullTime
}

func (NovelChapterSummaryTaskStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	table, err := summaryTaskTable(ctx, source)
	if err != nil {
		return BatchResult{}, err
	}
	if table == "" {
		return BatchResult{Done: true, Metadata: map[string]any{"skipped": "table_not_found"}}, nil
	}
	last, err := parseCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	rows, err := source.QueryContext(ctx, fmt.Sprintf(`SELECT id,status,COALESCE(stop_requested,0),COALESCE(total_count,0),COALESCE(processed_count,0),COALESCE(success_count,0),COALESCE(fail_count,0),current_result_id,current_book_id,current_chapter_id,COALESCE(error_summary,''),COALESCE(latest_error_message,''),start_time,end_time,create_time,update_time FROM %s WHERE id>? ORDER BY id LIMIT ?`, table), last, limit)
	if err != nil {
		return BatchResult{}, err
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": table, "latestRawResponseOmitted": true}}
	for rows.Next() {
		var item legacySummaryTask
		if err := rows.Scan(&item.id, &item.status, &item.stop, &item.total, &item.processed, &item.success, &item.fail, &item.resultID, &item.bookID, &item.chapterID, &item.errorSummary, &item.latestError, &item.start, &item.end, &item.created, &item.updated); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(item.id, 10)
		status, interrupted, ok := mapLegacySummaryTaskStatus(item.status)
		if !ok {
			result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "INVALID_SUMMARY_TASK_STATUS", "legacy summary task status is unsupported"))
			continue
		}
		if interrupted {
			status = "failed"
			if strings.TrimSpace(item.errorSummary) == "" {
				item.errorSummary = "legacy summary task was interrupted during cutover"
			}
		}
		key := "moonbook-v1:" + table + ":" + result.NextCursor
		if err := checkLegacyKey(ctx, target, "novel_chapter_summary_task", item.id, key); err != nil {
			result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "SOURCE_KEY_CONFLICT", err.Error()))
			continue
		}
		var currentResult, currentBook, currentChapter any
		if item.resultID.Valid {
			var exists bool
			if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_chapter_clean_result WHERE id=$1)`, item.resultID.Int64).Scan(&exists); err != nil {
				return BatchResult{}, err
			}
			if exists {
				currentResult = item.resultID.Int64
			} else {
				result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "CURRENT_RESULT_NOT_FOUND", "legacy summary task current result was not migrated"))
			}
		}
		if item.bookID.Valid {
			var exists bool
			if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_books WHERE id=$1)`, item.bookID.Int64).Scan(&exists); err != nil {
				return BatchResult{}, err
			}
			if exists {
				currentBook = item.bookID.Int64
			} else {
				result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "CURRENT_BOOK_NOT_FOUND", "legacy summary task current book was not migrated"))
			}
		}
		if item.chapterID.Valid {
			var exists bool
			if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_chapters WHERE id=$1)`, item.chapterID.Int64).Scan(&exists); err != nil {
				return BatchResult{}, err
			}
			if exists {
				currentChapter = item.chapterID.Int64
			} else {
				result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "CURRENT_CHAPTER_NOT_FOUND", "legacy summary task current chapter was not migrated"))
			}
		}
		_, err = target.ExecContext(ctx, `INSERT INTO novel_chapter_summary_task(id,status,automatic,total_count,processed_count,success_count,fail_count,current_result_id,current_book_id,current_chapter_id,stop_requested,operator_name,error_summary,latest_error_message,latest_raw_response,raw_response_expires_at,started_at,finished_at,created_at,updated_at) VALUES($1,$2,false,$3,$4,$5,$6,$7,$8,$9,$10,'',$11,$12,'',NULL,COALESCE($13,now()),$14,COALESCE($15,now()),COALESCE($16,now())) ON CONFLICT(id) DO UPDATE SET status=EXCLUDED.status,total_count=EXCLUDED.total_count,processed_count=EXCLUDED.processed_count,success_count=EXCLUDED.success_count,fail_count=EXCLUDED.fail_count,current_result_id=EXCLUDED.current_result_id,current_book_id=EXCLUDED.current_book_id,current_chapter_id=EXCLUDED.current_chapter_id,stop_requested=EXCLUDED.stop_requested,error_summary=EXCLUDED.error_summary,latest_error_message=EXCLUDED.latest_error_message,latest_raw_response='',raw_response_expires_at=NULL,started_at=EXCLUDED.started_at,finished_at=EXCLUDED.finished_at,updated_at=EXCLUDED.updated_at`, item.id, status, item.total, item.processed, item.success, item.fail, currentResult, currentBook, currentChapter, item.stop != 0, strings.TrimSpace(item.errorSummary), strings.TrimSpace(item.latestError), item.start, item.end, item.created, item.updated)
		if err != nil {
			return BatchResult{}, fmt.Errorf("upsert summary task %d: %w", item.id, err)
		}
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	if _, err = target.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('novel_chapter_summary_task','id'),GREATEST(COALESCE((SELECT max(id) FROM novel_chapter_summary_task),1),1),true)`); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

func summaryTaskTable(ctx context.Context, source *sql.DB) (string, error) {
	for _, name := range []string{"novel_chapter_summary_task_log", "novel_chapter_summary_task"} {
		ok, err := sourceTableExists(ctx, source, name)
		if err != nil {
			return "", err
		}
		if ok {
			return name, nil
		}
	}
	return "", nil
}
func mapLegacySummaryTaskStatus(v string) (string, bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "running", "pending", "processing":
		return "failed", true, true
	case "completed", "success", "succeeded":
		return "completed", false, true
	case "stopped", "cancelled", "canceled":
		return "stopped", false, true
	case "failed", "error":
		return "failed", false, true
	default:
		return "", false, false
	}
}
func resultDone(result BatchResult) BatchResult { result.Done = true; return result }
func resolveLegacyAIConfig(ctx context.Context, target *sql.Tx, aiID, fallbackID int64) (int64, int64, string, error) {
	var resolved int64
	warning := ""
	if aiID > 0 {
		if err := target.QueryRowContext(ctx, `SELECT id FROM novel_ai_config WHERE id=$1`, aiID).Scan(&resolved); err != nil && err != sql.ErrNoRows {
			return 0, 0, "", err
		}
	}
	if resolved == 0 {
		if err := target.QueryRowContext(ctx, `SELECT id FROM novel_ai_config ORDER BY id LIMIT 2`).Scan(&resolved); err != nil {
			return 0, 0, "", fmt.Errorf("no migrated AI config for summary config: %w", err)
		}
		warning = "legacy summary config had no valid AI config ID; selected the first migrated config"
	}
	fallback := int64(0)
	if fallbackID > 0 {
		var found int64
		if err := target.QueryRowContext(ctx, `SELECT id FROM novel_ai_config WHERE id=$1`, fallbackID).Scan(&found); err == nil && found != resolved {
			fallback = found
		} else if err != nil && err != sql.ErrNoRows {
			return 0, 0, "", err
		}
	}
	return resolved, fallback, warning, nil
}
