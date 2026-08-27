package legacymigrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
)

type NovelChapterCleanTasksStage struct{}

func (NovelChapterCleanTasksStage) Name() string { return "novel-chapter-clean-tasks" }

type legacyCleanTask struct {
	id, bookID                                                                         int64
	bookName, status, operator, errorSummary                                           string
	force, total, processed, success, discard, fail, skip, allDiscarded, stopRequested int
	start, end, created, updated                                                       sql.NullTime
}

func (NovelChapterCleanTasksStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	table, err := legacyCleanTable(ctx, source, "task")
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
	operatorColumn, err := legacyColumn(ctx, source, table, "operator_name", "operator")
	if err != nil {
		return BatchResult{}, err
	}
	rows, err := source.QueryContext(ctx, fmt.Sprintf(`SELECT id,book_id,COALESCE(book_name,''),status,COALESCE(force_reclean,0),COALESCE(total_count,0),COALESCE(processed_count,0),COALESCE(success_count,0),COALESCE(discard_count,0),COALESCE(fail_count,0),COALESCE(skip_count,0),COALESCE(all_chapters_discarded,0),COALESCE(stop_requested,0),COALESCE(%s,''),COALESCE(error_summary,''),start_time,end_time,create_time,update_time FROM %s WHERE id>? ORDER BY id LIMIT ?`, operatorColumn, table), last, limit)
	if err != nil {
		return BatchResult{}, fmt.Errorf("query legacy clean tasks: %w", err)
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": table}}
	for rows.Next() {
		var item legacyCleanTask
		if err := rows.Scan(&item.id, &item.bookID, &item.bookName, &item.status, &item.force, &item.total, &item.processed, &item.success, &item.discard, &item.fail, &item.skip, &item.allDiscarded, &item.stopRequested, &item.operator, &item.errorSummary, &item.start, &item.end, &item.created, &item.updated); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(item.id, 10)
		key := "moonbook-v1:" + table + ":" + result.NextCursor
		if item.id <= 0 || item.bookID <= 0 || item.total < 0 || item.processed < 0 || item.success < 0 || item.discard < 0 || item.fail < 0 || item.skip < 0 {
			result.Errors = append(result.Errors, cleanError(table, result.NextCursor, "INVALID_CLEAN_TASK", "legacy clean task fields are invalid"))
			continue
		}
		var bookExists bool
		if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_books WHERE id=$1 AND deleted_at IS NULL)`, item.bookID).Scan(&bookExists); err != nil {
			return BatchResult{}, err
		}
		if !bookExists {
			result.Errors = append(result.Errors, cleanError(table, result.NextCursor, "BOOK_NOT_FOUND", "legacy clean task references a missing target book"))
			continue
		}
		status, interrupted, ok := mapLegacyCleanTaskStatus(item.status)
		if !ok {
			result.Errors = append(result.Errors, cleanError(table, result.NextCursor, "INVALID_CLEAN_TASK_STATUS", "legacy clean task status is unsupported"))
			continue
		}
		if interrupted {
			status = "failed"
			if strings.TrimSpace(item.errorSummary) == "" {
				item.errorSummary = "legacy clean task was interrupted during cutover"
			}
		}
		if err := checkLegacyKey(ctx, target, "novel_chapter_clean_task", item.id, key); err != nil {
			result.Errors = append(result.Errors, cleanError(table, result.NextCursor, "SOURCE_KEY_CONFLICT", err.Error()))
			continue
		}
		_, err = target.ExecContext(ctx, `INSERT INTO novel_chapter_clean_task(id,book_id,book_name,status,force_reclean,total_count,processed_count,success_count,discard_count,fail_count,skip_count,all_chapters_discarded,stop_requested,operator_name,error_summary,started_at,finished_at,created_at,updated_at,legacy_source_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,COALESCE($16,now()),$17,COALESCE($18,now()),COALESCE($19,now()),$20) ON CONFLICT(id) DO UPDATE SET status=EXCLUDED.status,book_name=EXCLUDED.book_name,force_reclean=EXCLUDED.force_reclean,total_count=EXCLUDED.total_count,processed_count=EXCLUDED.processed_count,success_count=EXCLUDED.success_count,discard_count=EXCLUDED.discard_count,fail_count=EXCLUDED.fail_count,skip_count=EXCLUDED.skip_count,all_chapters_discarded=EXCLUDED.all_chapters_discarded,stop_requested=EXCLUDED.stop_requested,operator_name=EXCLUDED.operator_name,error_summary=EXCLUDED.error_summary,started_at=EXCLUDED.started_at,finished_at=EXCLUDED.finished_at,updated_at=EXCLUDED.updated_at,legacy_source_key=EXCLUDED.legacy_source_key`, item.id, item.bookID, strings.TrimSpace(item.bookName), status, item.force != 0, item.total, item.processed, item.success, item.discard, item.fail, item.skip, item.allDiscarded != 0, item.stopRequested != 0, strings.TrimSpace(item.operator), strings.TrimSpace(item.errorSummary), item.start, item.end, item.created, item.updated, key)
		if err != nil {
			return BatchResult{}, fmt.Errorf("upsert clean task %d: %w", item.id, err)
		}
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	if _, err = target.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('novel_chapter_clean_task','id'),GREATEST(COALESCE((SELECT max(id) FROM novel_chapter_clean_task),1),1),true)`); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

type NovelChapterCleanResultsStage struct{ Objects *objectstore.Service }

func (NovelChapterCleanResultsStage) Name() string { return "novel-chapter-clean-results" }

type legacyCleanResult struct {
	id, taskID, bookID, chapterID          int64
	sourceNo                               int
	contentType                            string
	novelBody, removed                     sql.NullBool
	title, text, raw, status, errorMessage string
	wordCount                              int
	confidence                             sql.NullFloat64
	active                                 int
	created, updated                       sql.NullTime
}

func (stage NovelChapterCleanResultsStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	if stage.Objects == nil {
		return BatchResult{}, errors.New("chapter clean result migration requires object storage")
	}
	table, err := legacyCleanTable(ctx, source, "result")
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
	chapterColumn, err := legacyColumn(ctx, source, table, "chapter_id", "index_id")
	if err != nil {
		return BatchResult{}, err
	}
	sourceNoColumn, err := legacyColumn(ctx, source, table, "source_chapter_no", "source_index_num")
	if err != nil {
		return BatchResult{}, err
	}
	titleColumn, err := legacyColumn(ctx, source, table, "cleaned_chapter_name", "chapter_title")
	if err != nil {
		return BatchResult{}, err
	}
	rows, err := source.QueryContext(ctx, fmt.Sprintf(`SELECT id,COALESCE(task_id,0),book_id,%s,COALESCE(%s,0),COALESCE(content_type,''),is_novel_body,COALESCE(%s,''),COALESCE(cleaned_text,''),COALESCE(cleaned_word_count,0),removed_non_novel,confidence,COALESCE(raw_response,''),status,COALESCE(error_message,''),COALESCE(active,1),create_time,update_time FROM %s WHERE id>? ORDER BY id LIMIT ?`, chapterColumn, sourceNoColumn, titleColumn, table), last, limit)
	if err != nil {
		return BatchResult{}, fmt.Errorf("query legacy clean results: %w", err)
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": table, "rawResponseOmitted": true, "objectCount": 0}}
	for rows.Next() {
		var item legacyCleanResult
		if err := rows.Scan(&item.id, &item.taskID, &item.bookID, &item.chapterID, &item.sourceNo, &item.contentType, &item.novelBody, &item.title, &item.text, &item.wordCount, &item.removed, &item.confidence, &item.raw, &item.status, &item.errorMessage, &item.active, &item.created, &item.updated); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(item.id, 10)
		key := "moonbook-v1:" + table + ":" + result.NextCursor
		if item.id <= 0 || item.bookID <= 0 || item.chapterID <= 0 || !utf8.ValidString(item.text) {
			result.Errors = append(result.Errors, cleanError(table, result.NextCursor, "INVALID_CLEAN_RESULT", "legacy clean result identity or text is invalid"))
			continue
		}
		if err := checkLegacyKey(ctx, target, "novel_chapter_clean_result", item.id, key); err != nil {
			result.Errors = append(result.Errors, cleanError(table, result.NextCursor, "SOURCE_KEY_CONFLICT", err.Error()))
			continue
		}
		var alreadyMigrated bool
		if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_chapter_clean_result WHERE id=$1 AND legacy_source_key=$2)`, item.id, key).Scan(&alreadyMigrated); err != nil {
			return BatchResult{}, err
		}
		if alreadyMigrated {
			continue
		}
		var taskExists, chapterExists bool
		if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_chapter_clean_task WHERE id=$1),EXISTS(SELECT 1 FROM novel_chapters WHERE id=$2 AND book_id=$3)`, item.taskID, item.chapterID, item.bookID).Scan(&taskExists, &chapterExists); err != nil {
			return BatchResult{}, err
		}
		if item.taskID <= 0 || !taskExists {
			result.Errors = append(result.Errors, cleanError(table, result.NextCursor, "TASK_NOT_FOUND", "legacy clean result task was not migrated"))
			continue
		}
		if !chapterExists {
			result.Errors = append(result.Errors, cleanError(table, result.NextCursor, "CHAPTER_NOT_FOUND", "legacy clean result chapter/book association is missing"))
			continue
		}
		status, ok := mapLegacyCleanResultStatus(item.status)
		if !ok {
			result.Errors = append(result.Errors, cleanError(table, result.NextCursor, "INVALID_CLEAN_RESULT_STATUS", "legacy clean result status is unsupported"))
			continue
		}
		var original sql.NullInt64
		_ = target.QueryRowContext(ctx, `SELECT id FROM novel_objects WHERE object_kind='chapter_content' AND book_id=$1 AND owner_id=$2 AND state='active' ORDER BY version DESC LIMIT 1`, item.bookID, item.chapterID).Scan(&original)
		if item.active == 0 || status != "success" || strings.TrimSpace(item.text) == "" {
			if item.active != 0 && status == "success" && strings.TrimSpace(item.text) == "" {
				status = "failed"
				item.errorMessage = "legacy successful result has no cleaned text"
				result.Errors = append(result.Errors, cleanError(table, result.NextCursor, "CLEANED_TEXT_NOT_FOUND", item.errorMessage))
			}
			if err := insertCleanResult(ctx, target, item, status, original, nil, key); err != nil {
				return BatchResult{}, err
			}
			continue
		}
		digest := sha256.Sum256([]byte(item.text))
		fingerprint := hex.EncodeToString(digest[:])
		obj, err := stage.Objects.UploadVerifiedWithOptions(ctx, objectstore.Target{Kind: objectstore.KindChapterClean, BookID: item.bookID, OwnerID: item.chapterID}, []byte(item.text), "text/plain; charset=utf-8", objectstore.UploadOptions{Source: "legacy", SourceFingerprint: fingerprint})
		if err != nil {
			result.Errors = append(result.Errors, cleanError(table, result.NextCursor, "CLEANED_OBJECT_UPLOAD_FAILED", "legacy cleaned text upload or verification failed"))
			continue
		}
		if obj.State == objectstore.StateActive {
			if err := insertCleanResult(ctx, target, item, status, original, &obj.ID, key); err != nil {
				return BatchResult{}, err
			}
		} else {
			err = stage.Objects.ActivateWithTx(ctx, obj.ID, func(tx *sql.Tx, _ objectstore.Target) error {
				return insertCleanResult(ctx, tx, item, status, original, &obj.ID, key)
			})
			if err != nil {
				result.Errors = append(result.Errors, cleanError(table, result.NextCursor, "CLEAN_RESULT_ACTIVATE_FAILED", err.Error()))
				continue
			}
		}
		result.Metadata["objectCount"] = result.Metadata["objectCount"].(int) + 1
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	if _, err = target.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('novel_chapter_clean_result','id'),GREATEST(COALESCE((SELECT max(id) FROM novel_chapter_clean_result),1),1),true)`); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

func legacyCleanTable(ctx context.Context, source *sql.DB, kind string) (string, error) {
	for _, name := range []string{"novel_chapter_clean_" + kind, "ai_chapter_clean_" + kind} {
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

func legacyColumn(ctx context.Context, source *sql.DB, table string, candidates ...string) (string, error) {
	for _, column := range candidates {
		var exists bool
		if err := source.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?)`, table, column).Scan(&exists); err != nil {
			return "", err
		}
		if exists {
			return column, nil
		}
	}
	return "", fmt.Errorf("legacy table %s has none of the supported columns %s", table, strings.Join(candidates, ","))
}

func mapLegacyCleanTaskStatus(v string) (string, bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "running", "processing", "pending", "queued":
		return "failed", true, true
	case "completed", "success", "done":
		return "completed", false, true
	case "stopped", "cancelled", "canceled":
		return "stopped", false, true
	case "failed", "error":
		return "failed", false, true
	default:
		return "", false, false
	}
}
func mapLegacyCleanResultStatus(v string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "success", "succeeded", "completed":
		return "success", true
	case "discarded", "manual_discarded":
		return "discarded", true
	case "failed", "error":
		return "failed", true
	case "skipped":
		return "skipped", true
	case "expired":
		return "expired", true
	case "pending_review", "pending":
		return "pending_review", true
	default:
		return "", false
	}
}
func cleanError(table, id, code, msg string) RecordError {
	return RecordError{SourceTable: table, SourceID: id, Code: code, Message: msg, Retryable: false}
}
func insertCleanResult(ctx context.Context, tx *sql.Tx, item legacyCleanResult, status string, original sql.NullInt64, cleaned *int64, key string) error {
	if item.active != 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE novel_chapter_clean_result SET active=false,status='expired',updated_at=now() WHERE chapter_id=$1 AND active`, item.chapterID); err != nil {
			return err
		}
	}
	var orig any
	if original.Valid {
		orig = original.Int64
	}
	var clean any
	if cleaned != nil {
		clean = *cleaned
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO novel_chapter_clean_result(id,task_id,book_id,chapter_id,source_chapter_no,content_type,is_novel_body,cleaned_chapter_name,cleaned_object_id,original_object_id,cleaned_word_count,removed_non_novel,confidence,status,error_message,active,created_at,updated_at,legacy_source_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,COALESCE($17,now()),COALESCE($18,now()),$19) ON CONFLICT(id) DO NOTHING`, item.id, item.taskID, item.bookID, item.chapterID, item.sourceNo, strings.TrimSpace(item.contentType), item.novelBody, item.title, clean, orig, item.wordCount, item.removed, item.confidence, status, strings.TrimSpace(item.errorMessage), item.active != 0, item.created, item.updated, key)
	return err
}
