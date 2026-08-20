package legacymigrate

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// NovelCrawlImportTasksStage migrates historical forum import facts and creates
// a non-running platform job for each task. Historical running tasks are marked
// failed with an interruption reason so cutover never resumes external fetches
// without an operator decision.
type NovelCrawlImportTasksStage struct{}

func (NovelCrawlImportTasksStage) Name() string { return "novel-crawl-import-tasks" }

type legacyCrawlImportTask struct {
	id, sourceID, boardID                          int64
	sourceName, boardName, threadID, title         string
	displayTitle, threadURL, importMode, mergeMode string
	status, qualityStatus                          string
	targetBookID                                   sql.NullInt64
	total, imported, empty, duplicate              int64
	qualitySummary, failReason, operatorName       string
	start, end, created, updated, threadCreated    sql.NullTime
}

func (NovelCrawlImportTasksStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	lastID, err := parseCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	exists, err := sourceTableExists(ctx, source, "novel_crawl_import_task")
	if err != nil {
		return BatchResult{}, err
	}
	if !exists {
		return BatchResult{Done: true, Metadata: map[string]any{"source": "novel_crawl_import_task", "skipped": "table_not_found"}}, nil
	}
	rows, err := source.QueryContext(ctx, `SELECT id,COALESCE(source_id,0),COALESCE(source_name,''),COALESCE(board_id,0),COALESCE(board_name,''),COALESCE(forum_thread_id,''),COALESCE(thread_title,''),COALESCE(display_title,''),COALESCE(thread_url,''),COALESCE(thread_create_time,NULL),COALESCE(target_book_id,0),COALESCE(import_mode,'create'),COALESCE(merge_strategy,'source_thread'),COALESCE(status,'pending'),COALESCE(total_chapter_count,0),COALESCE(imported_chapter_count,0),COALESCE(quality_status,'pending'),COALESCE(quality_summary,''),COALESCE(empty_chapter_count,0),COALESCE(duplicate_chapter_count,0),COALESCE(fail_reason,''),COALESCE(operator_name,''),start_time,end_time,create_time,update_time FROM novel_crawl_import_task WHERE id>? ORDER BY id LIMIT ?`, lastID, limit)
	if err != nil {
		return BatchResult{}, fmt.Errorf("query legacy crawl import tasks: %w", err)
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": "novel_crawl_import_task", "runningInterrupted": 0}}
	for rows.Next() {
		var item legacyCrawlImportTask
		if err := rows.Scan(&item.id, &item.sourceID, &item.sourceName, &item.boardID, &item.boardName, &item.threadID, &item.title, &item.displayTitle, &item.threadURL, &item.threadCreated, &item.targetBookID, &item.importMode, &item.mergeMode, &item.status, &item.total, &item.imported, &item.qualityStatus, &item.qualitySummary, &item.empty, &item.duplicate, &item.failReason, &item.operatorName, &item.start, &item.end, &item.created, &item.updated); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(item.id, 10)
		key := "moonbook-v1:novel_crawl_import_task:" + result.NextCursor
		if code, message := validateLegacyImportTask(item); code != "" {
			result.Errors = append(result.Errors, crawlError("novel_crawl_import_task", result.NextCursor, code, message))
			continue
		}
		status, interrupted, ok := mapLegacyImportStatus(item.status)
		if !ok {
			result.Errors = append(result.Errors, crawlError("novel_crawl_import_task", result.NextCursor, "INVALID_IMPORT_STATUS", "legacy import task status is unsupported"))
			continue
		}
		quality, ok := mapLegacyQualityStatus(item.qualityStatus)
		if !ok {
			result.Errors = append(result.Errors, crawlError("novel_crawl_import_task", result.NextCursor, "INVALID_QUALITY_STATUS", "legacy import quality status is unsupported"))
			continue
		}
		var candidateID, sourceID, boardID, sourceName, boardName, threadURL string
		err = target.QueryRowContext(ctx, `SELECT id::text,source_id::text,board_id::text,source_name,board_name,thread_url FROM novel_crawl_thread_candidate WHERE source_id=$1 AND board_id=$2 AND forum_thread_id=$3 ORDER BY id LIMIT 1`, item.sourceID, item.boardID, strings.TrimSpace(item.threadID)).Scan(&candidateID, &sourceID, &boardID, &sourceName, &boardName, &threadURL)
		if err != nil {
			result.Errors = append(result.Errors, crawlError("novel_crawl_import_task", result.NextCursor, "CANDIDATE_NOT_FOUND", "legacy import task has no migrated forum candidate"))
			continue
		}
		candidateIDValue, parseCandidateErr := strconv.ParseInt(candidateID, 10, 64)
		sourceIDValue, parseSourceErr := strconv.ParseInt(sourceID, 10, 64)
		boardIDValue, parseBoardErr := strconv.ParseInt(boardID, 10, 64)
		if parseCandidateErr != nil || parseSourceErr != nil || parseBoardErr != nil {
			result.Errors = append(result.Errors, crawlError("novel_crawl_import_task", result.NextCursor, "INVALID_TARGET_RELATION", "migrated candidate relation contains an invalid ID"))
			continue
		}
		bookValue := nullableID(item.targetBookID)
		var bookArg any
		if bookValue != nil {
			bookArg = *bookValue
		}
		if bookValue != nil {
			var found bool
			if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_books WHERE id=$1 AND deleted_at IS NULL)`, *bookValue).Scan(&found); err != nil {
				return BatchResult{}, err
			}
			if !found {
				result.Errors = append(result.Errors, crawlError("novel_crawl_import_task", result.NextCursor, "TARGET_BOOK_NOT_FOUND", "target book was not migrated; task kept without target_book_id"))
				bookArg = nil
			}
		}
		payload, _ := json.Marshal(map[string]string{"importTaskId": result.NextCursor, "candidateId": candidateID})
		jobStatus := status
		if interrupted {
			jobStatus = "failed"
			result.Metadata["runningInterrupted"] = result.Metadata["runningInterrupted"].(int) + 1
		}
		var jobID int64
		if err := target.QueryRowContext(ctx, `INSERT INTO platform_jobs(module,job_type,idempotency_key,status,payload,max_attempts,finished_at,last_error_code,last_error_message) VALUES('novel','forum_import',$1,$2,$3,3,CASE WHEN $2 IN ('succeeded','failed','cancelled') THEN now() END,CASE WHEN $2='failed' THEN 'LEGACY_INTERRUPTED' END,CASE WHEN $2='failed' THEN 'legacy import task was not resumed automatically' END) ON CONFLICT(module,job_type,idempotency_key) DO UPDATE SET status=EXCLUDED.status,payload=EXCLUDED.payload,finished_at=EXCLUDED.finished_at,last_error_code=EXCLUDED.last_error_code,last_error_message=EXCLUDED.last_error_message RETURNING id`, "import-task:"+result.NextCursor, jobStatus, payload).Scan(&jobID); err != nil {
			return BatchResult{}, fmt.Errorf("upsert legacy forum import job %d: %w", item.id, err)
		}
		_, err = target.ExecContext(ctx, `INSERT INTO novel_crawl_import_task(id,candidate_id,platform_job_id,source_id,source_name,board_id,board_name,forum_thread_id,thread_title,display_title,thread_url,thread_created_at,target_book_id,import_mode,merge_strategy,status,total_chapter_count,imported_chapter_count,quality_status,quality_summary,empty_chapter_count,duplicate_chapter_count,fail_reason,operator_name,start_time,end_time,created_at,updated_at,legacy_source_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,COALESCE($28,now()),COALESCE($29,now()),$30) ON CONFLICT(id) DO UPDATE SET candidate_id=EXCLUDED.candidate_id,platform_job_id=EXCLUDED.platform_job_id,target_book_id=EXCLUDED.target_book_id,status=EXCLUDED.status,total_chapter_count=EXCLUDED.total_chapter_count,imported_chapter_count=EXCLUDED.imported_chapter_count,quality_status=EXCLUDED.quality_status,quality_summary=EXCLUDED.quality_summary,empty_chapter_count=EXCLUDED.empty_chapter_count,duplicate_chapter_count=EXCLUDED.duplicate_chapter_count,fail_reason=EXCLUDED.fail_reason,operator_name=EXCLUDED.operator_name,start_time=EXCLUDED.start_time,end_time=EXCLUDED.end_time,updated_at=EXCLUDED.updated_at,legacy_source_key=EXCLUDED.legacy_source_key`, item.id, candidateIDValue, jobID, sourceIDValue, sourceName, boardIDValue, boardName, strings.TrimSpace(item.threadID), strings.TrimSpace(item.title), strings.TrimSpace(item.displayTitle), strings.TrimSpace(item.threadURL), item.threadCreated, bookArg, strings.TrimSpace(item.importMode), strings.TrimSpace(item.mergeMode), status, item.total, item.imported, quality, strings.TrimSpace(item.qualitySummary), item.empty, item.duplicate, strings.TrimSpace(item.failReason), strings.TrimSpace(item.operatorName), item.start, item.end, item.created, item.updated, key)
		if err != nil {
			return BatchResult{}, fmt.Errorf("upsert legacy forum import task %d: %w", item.id, err)
		}
		if _, err := target.ExecContext(ctx, `UPDATE novel_crawl_thread_candidate SET import_task_id=$1,status=CASE WHEN $2='succeeded' THEN 'imported' WHEN $2 IN ('failed','cancelled') THEN 'failed' ELSE 'importing' END,updated_at=now() WHERE id=$3`, item.id, status, candidateIDValue); err != nil {
			return BatchResult{}, err
		}
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	if err := syncIdentitySequence(ctx, target, "novel_crawl_import_task"); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

func validateLegacyImportTask(item legacyCrawlImportTask) (string, string) {
	if item.id <= 0 || item.sourceID <= 0 || item.boardID <= 0 || strings.TrimSpace(item.threadID) == "" || strings.TrimSpace(item.title) == "" || strings.TrimSpace(item.threadURL) == "" {
		return "INVALID_IMPORT_IDENTITY", "legacy import task identity or relation is invalid"
	}
	if item.total < 0 || item.imported < 0 || item.empty < 0 || item.duplicate < 0 {
		return "INVALID_IMPORT_COUNT", "legacy import counters must be non-negative"
	}
	return "", ""
}

func mapLegacyImportStatus(value string) (string, bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pending":
		return "pending", false, true
	case "running":
		return "failed", true, true
	case "success", "succeeded":
		return "succeeded", false, true
	case "failed":
		return "failed", false, true
	case "cancelled", "canceled", "stopped":
		return "cancelled", false, true
	default:
		return "", false, false
	}
}

func mapLegacyQualityStatus(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pending", "passed", "warning", "failed":
		return strings.ToLower(strings.TrimSpace(value)), true
	default:
		return "", false
	}
}
