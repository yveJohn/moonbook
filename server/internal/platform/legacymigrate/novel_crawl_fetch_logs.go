package legacymigrate

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// NovelCrawlFetchLogsStage migrates append-only crawl audit records after the
// source, candidate, and import-task stages. Missing optional relations are
// retained as null with a structured migration error.
type NovelCrawlFetchLogsStage struct{}

func (NovelCrawlFetchLogsStage) Name() string { return "novel-crawl-fetch-logs" }

type legacyCrawlFetchLog struct {
	id, taskID, sourceID, boardID, targetBookID  sql.NullInt64
	sourceName, boardName, threadURL, requestURL string
	stage, status, message, detailJSON           string
	httpStatus, responseBytes, elapsedMS, count  sql.NullInt64
	created                                      sql.NullTime
}

func (NovelCrawlFetchLogsStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	lastID, err := parseCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	exists, err := sourceTableExists(ctx, source, "novel_crawl_fetch_log")
	if err != nil {
		return BatchResult{}, err
	}
	if !exists {
		return BatchResult{Done: true, Metadata: map[string]any{"source": "novel_crawl_fetch_log", "skipped": "table_not_found"}}, nil
	}
	rows, err := source.QueryContext(ctx, `SELECT id,task_id,source_id,COALESCE(source_name,''),board_id,COALESCE(board_name,''),COALESCE(thread_url,''),COALESCE(request_url,''),COALESCE(stage,''),COALESCE(status,''),http_status,response_bytes,elapsed_ms,item_count,target_book_id,COALESCE(message,''),COALESCE(detail_json,''),create_time FROM novel_crawl_fetch_log WHERE id>? ORDER BY id LIMIT ?`, lastID, limit)
	if err != nil {
		return BatchResult{}, fmt.Errorf("query legacy crawl fetch logs: %w", err)
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": "novel_crawl_fetch_log"}}
	for rows.Next() {
		var item legacyCrawlFetchLog
		if err := rows.Scan(&item.id, &item.taskID, &item.sourceID, &item.sourceName, &item.boardID, &item.boardName, &item.threadURL, &item.requestURL, &item.stage, &item.status, &item.httpStatus, &item.responseBytes, &item.elapsedMS, &item.count, &item.targetBookID, &item.message, &item.detailJSON, &item.created); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(item.id.Int64, 10)
		key := "moonbook-v1:novel_crawl_fetch_log:" + result.NextCursor
		if code, message := validateLegacyFetchLog(item); code != "" {
			result.Errors = append(result.Errors, crawlError("novel_crawl_fetch_log", result.NextCursor, code, message))
			continue
		}
		stage, ok := mapLegacyFetchStage(item.stage, item.taskID.Valid && item.taskID.Int64 > 0)
		if !ok {
			result.Errors = append(result.Errors, crawlError("novel_crawl_fetch_log", result.NextCursor, "INVALID_FETCH_STAGE", "legacy fetch log stage is unsupported"))
			continue
		}
		status, ok := mapLegacyFetchStatus(item.status)
		if !ok {
			result.Errors = append(result.Errors, crawlError("novel_crawl_fetch_log", result.NextCursor, "INVALID_FETCH_STATUS", "legacy fetch log status is unsupported"))
			continue
		}
		if item.detailJSON != "" && !json.Valid([]byte(item.detailJSON)) {
			result.Errors = append(result.Errors, crawlError("novel_crawl_fetch_log", result.NextCursor, "INVALID_DETAIL_JSON", "legacy fetch log detail is not valid JSON"))
			continue
		}
		args := []any{nullableInt(item.taskID), nullableInt(item.sourceID), strings.TrimSpace(item.sourceName), nullableInt(item.boardID), strings.TrimSpace(item.boardName), nullIfEmpty(item.threadURL), nullIfEmpty(item.requestURL), stage, status, nullableInt(item.httpStatus), nullableInt(item.responseBytes), nullableInt(item.elapsedMS), nullableInt(item.count), nullableInt(item.targetBookID), strings.TrimSpace(item.message), nullJSON(item.detailJSON), item.created, key}
		for _, relation := range []struct {
			table string
			value sql.NullInt64
			code  string
		}{{"novel_crawl_import_task", item.taskID, "TASK_NOT_FOUND"}, {"novel_crawl_forum_source", item.sourceID, "SOURCE_NOT_FOUND"}, {"novel_crawl_forum_board", item.boardID, "BOARD_NOT_FOUND"}, {"novel_books", item.targetBookID, "TARGET_BOOK_NOT_FOUND"}} {
			if !relation.value.Valid || relation.value.Int64 <= 0 {
				continue
			}
			var found bool
			if err := target.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM "+relation.table+" WHERE id=$1)", relation.value.Int64).Scan(&found); err != nil {
				return BatchResult{}, err
			}
			if !found {
				result.Errors = append(result.Errors, crawlError("novel_crawl_fetch_log", result.NextCursor, relation.code, "legacy fetch log relation was not migrated; relation stored as null"))
				switch relation.table {
				case "novel_crawl_import_task":
					args[0] = nil
				case "novel_crawl_forum_source":
					args[1] = nil
				case "novel_crawl_forum_board":
					args[3] = nil
				case "novel_books":
					args[13] = nil
				}
			}
		}
		_, err = target.ExecContext(ctx, `INSERT INTO novel_crawl_fetch_log(id,task_id,source_id,source_name,board_id,board_name,thread_url,request_url,stage,status,http_status,response_bytes,elapsed_ms,item_count,target_book_id,message,detail_json,created_at,legacy_source_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,COALESCE($18,now()),$19) ON CONFLICT(id) DO UPDATE SET task_id=EXCLUDED.task_id,source_id=EXCLUDED.source_id,source_name=EXCLUDED.source_name,board_id=EXCLUDED.board_id,board_name=EXCLUDED.board_name,thread_url=EXCLUDED.thread_url,request_url=EXCLUDED.request_url,stage=EXCLUDED.stage,status=EXCLUDED.status,http_status=EXCLUDED.http_status,response_bytes=EXCLUDED.response_bytes,elapsed_ms=EXCLUDED.elapsed_ms,item_count=EXCLUDED.item_count,target_book_id=EXCLUDED.target_book_id,message=EXCLUDED.message,detail_json=EXCLUDED.detail_json,created_at=EXCLUDED.created_at,legacy_source_key=EXCLUDED.legacy_source_key`, append([]any{item.id.Int64}, args...)...)
		if err != nil {
			return BatchResult{}, fmt.Errorf("upsert legacy crawl fetch log %d: %w", item.id.Int64, err)
		}
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	if err := syncIdentitySequence(ctx, target, "novel_crawl_fetch_log"); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

func validateLegacyFetchLog(item legacyCrawlFetchLog) (string, string) {
	if !item.id.Valid || item.id.Int64 <= 0 {
		return "INVALID_FETCH_ID", "legacy fetch log ID must be positive"
	}
	if item.httpStatus.Valid && (item.httpStatus.Int64 < 0 || item.httpStatus.Int64 > 999) {
		return "INVALID_HTTP_STATUS", "legacy HTTP status is outside the supported range"
	}
	for _, value := range []sql.NullInt64{item.responseBytes, item.elapsedMS, item.count} {
		if value.Valid && value.Int64 < 0 {
			return "INVALID_FETCH_COUNT", "legacy fetch log counters must be non-negative"
		}
	}
	return "", ""
}

func mapLegacyFetchStage(value string, hasTask bool) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "discover", "thread", "chapter", "import", "quality":
		return strings.ToLower(strings.TrimSpace(value)), true
	case "fetch", "rate_limit":
		if hasTask {
			return "thread", true
		}
		return "discover", true
	case "parse":
		return "chapter", true
	default:
		return "", false
	}
}
func mapLegacyFetchStatus(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "started", "succeeded", "failed", "skipped":
		return strings.ToLower(strings.TrimSpace(value)), true
	case "success":
		return "succeeded", true
	default:
		return "", false
	}
}
func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}
func nullJSON(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return []byte(value)
}
