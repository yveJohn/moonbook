package legacymigrate

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

type NovelBookMergeTasksStage struct{}

func (NovelBookMergeTasksStage) Name() string { return "novel-book-merge-tasks" }

type legacyMergeTask struct {
	id, targetBookID                                               int64
	targetBookName, status, contentPolicy, sortPolicy, archiveMode string
	operator, errorSummary                                         string
	sourceCount, chapterCount, included, excluded, duplicates      int
	start, end, created, updated                                   sql.NullTime
}

func (NovelBookMergeTasksStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	table, err := legacyMergeTable(ctx, source, "task")
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
	rows, err := source.QueryContext(ctx, fmt.Sprintf(`SELECT id,COALESCE(target_book_id,0),COALESCE(target_book_name,''),status,COALESCE(source_count,0),COALESCE(chapter_count,0),COALESCE(included_chapter_count,0),COALESCE(excluded_chapter_count,0),COALESCE(duplicate_chapter_count,0),COALESCE(content_policy,'clean_first'),COALESCE(sort_policy,'thread_create_time_then_chapter_no'),COALESCE(source_archive_mode,'unpublish_after_success'),COALESCE(operator_name,''),COALESCE(error_summary,''),start_time,end_time,create_time,update_time FROM %s WHERE id>? ORDER BY id LIMIT ?`, table), last, limit)
	if err != nil {
		return BatchResult{}, fmt.Errorf("query legacy merge tasks: %w", err)
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": table}}
	for rows.Next() {
		var item legacyMergeTask
		if err := rows.Scan(&item.id, &item.targetBookID, &item.targetBookName, &item.status, &item.sourceCount, &item.chapterCount, &item.included, &item.excluded, &item.duplicates, &item.contentPolicy, &item.sortPolicy, &item.archiveMode, &item.operator, &item.errorSummary, &item.start, &item.end, &item.created, &item.updated); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(item.id, 10)
		if item.id <= 0 || item.sourceCount < 0 || item.chapterCount < 0 || item.included < 0 || item.excluded < 0 || item.duplicates < 0 {
			result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "INVALID_MERGE_TASK", "legacy merge task identity or counts are invalid"))
			continue
		}
		status, interrupted, ok := mapLegacyMergeTaskStatus(item.status)
		if !ok {
			result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "INVALID_MERGE_TASK_STATUS", "legacy merge task status is unsupported"))
			continue
		}
		if interrupted {
			status = "failed"
			if strings.TrimSpace(item.errorSummary) == "" {
				item.errorSummary = "legacy merge task was interrupted during cutover"
			}
		}
		if status == "succeeded" && item.targetBookID <= 0 {
			status = "failed"
			item.errorSummary = "legacy successful merge has no target book"
			result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "TARGET_BOOK_NOT_FOUND", "successful legacy merge has no target book"))
		}
		if item.sourceCount < 2 || item.chapterCount <= 0 || item.included <= 0 || item.chapterCount != item.included+item.excluded {
			result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "INVALID_MERGE_TASK_COUNTS", "legacy merge task counts cannot satisfy target constraints"))
			continue
		}
		var targetBook any
		if item.targetBookID > 0 {
			var exists bool
			if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_books WHERE id=$1)`, item.targetBookID).Scan(&exists); err != nil {
				return BatchResult{}, err
			}
			if !exists {
				if status == "succeeded" {
					status = "failed"
				}
				result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "TARGET_BOOK_NOT_FOUND", "legacy merge target book was not migrated"))
			} else {
				targetBook = item.targetBookID
			}
		}
		key := "moonbook-v1:" + table + ":" + result.NextCursor
		if err := checkLegacyKey(ctx, target, "novel_book_merge_task", item.id, key); err != nil {
			result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "SOURCE_KEY_CONFLICT", err.Error()))
			continue
		}
		_, err = target.ExecContext(ctx, `INSERT INTO novel_book_merge_task(id,target_book_id,target_book_name,status,source_count,chapter_count,included_chapter_count,excluded_chapter_count,duplicate_chapter_count,content_policy,sort_policy,source_archive_mode,operator_name,error_summary,start_time,end_time,created_at,updated_at,legacy_source_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,COALESCE($15,now()),$16,COALESCE($17,now()),COALESCE($18,now()),$19) ON CONFLICT(id) DO UPDATE SET target_book_id=EXCLUDED.target_book_id,target_book_name=EXCLUDED.target_book_name,status=EXCLUDED.status,source_count=EXCLUDED.source_count,chapter_count=EXCLUDED.chapter_count,included_chapter_count=EXCLUDED.included_chapter_count,excluded_chapter_count=EXCLUDED.excluded_chapter_count,duplicate_chapter_count=EXCLUDED.duplicate_chapter_count,content_policy=EXCLUDED.content_policy,sort_policy=EXCLUDED.sort_policy,source_archive_mode=EXCLUDED.source_archive_mode,operator_name=EXCLUDED.operator_name,error_summary=EXCLUDED.error_summary,start_time=EXCLUDED.start_time,end_time=EXCLUDED.end_time,updated_at=EXCLUDED.updated_at,legacy_source_key=EXCLUDED.legacy_source_key`, item.id, targetBook, strings.TrimSpace(item.targetBookName), status, item.sourceCount, item.chapterCount, item.included, item.excluded, item.duplicates, strings.TrimSpace(item.contentPolicy), strings.TrimSpace(item.sortPolicy), strings.TrimSpace(item.archiveMode), strings.TrimSpace(item.operator), strings.TrimSpace(item.errorSummary), item.start, item.end, item.created, item.updated, key)
		if err != nil {
			return BatchResult{}, fmt.Errorf("upsert merge task %d: %w", item.id, err)
		}
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	if _, err = target.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('novel_book_merge_task','id'),GREATEST(COALESCE((SELECT max(id) FROM novel_book_merge_task),1),1),true)`); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

type NovelBookMergeSourcesStage struct{}

func (NovelBookMergeSourcesStage) Name() string { return "novel-book-merge-sources" }

type legacyMergeSource struct {
	id, taskID, bookID, importTaskID                                   int64
	bookName, authorName, sourceName, threadID, threadTitle, threadURL string
	sortTime                                                           sql.NullTime
	sortSource                                                         string
	order                                                              int
	oldStatus, archivedStatus                                          string
	archiveTime                                                        sql.NullTime
	created                                                            sql.NullTime
}

func (NovelBookMergeSourcesStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	table, err := legacyMergeTable(ctx, source, "source")
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
	rows, err := source.QueryContext(ctx, fmt.Sprintf(`SELECT s.id,s.task_id,s.source_book_id,COALESCE(s.source_book_name,''),COALESCE(s.source_author_name,''),COALESCE(s.source_import_task_id,0),COALESCE(s.source_thread_id,''),COALESCE(i.thread_title,''),COALESCE(s.source_thread_url,''),COALESCE(s.sort_time,i.create_time),COALESCE(s.sort_time_source,'import_task_create_time'),COALESCE(s.source_order,0),COALESCE(s.old_publish_status,''),COALESCE(s.archived_publish_status,''),s.archive_time,s.create_time FROM %s s LEFT JOIN novel_crawl_import_task i ON i.id=s.source_import_task_id WHERE s.id>? ORDER BY s.id LIMIT ?`, table), last, limit)
	if err != nil {
		return BatchResult{}, fmt.Errorf("query legacy merge sources: %w", err)
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": table}}
	for rows.Next() {
		var item legacyMergeSource
		if err := rows.Scan(&item.id, &item.taskID, &item.bookID, &item.bookName, &item.authorName, &item.importTaskID, &item.threadID, &item.threadTitle, &item.threadURL, &item.sortTime, &item.sortSource, &item.order, &item.oldStatus, &item.archivedStatus, &item.archiveTime, &item.created); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(item.id, 10)
		key := "moonbook-v1:" + table + ":" + result.NextCursor
		if item.id <= 0 || item.taskID <= 0 || item.bookID <= 0 || item.importTaskID <= 0 || item.order <= 0 || !item.sortTime.Valid || strings.TrimSpace(item.threadID) == "" || strings.TrimSpace(item.threadTitle) == "" || strings.TrimSpace(item.threadURL) == "" {
			result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "INVALID_MERGE_SOURCE", "legacy merge source is missing required forum provenance"))
			continue
		}
		if err := checkLegacyKey(ctx, target, "novel_book_merge_source", item.id, key); err != nil {
			result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "SOURCE_KEY_CONFLICT", err.Error()))
			continue
		}
		var taskOK, bookOK, importOK bool
		if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_book_merge_task WHERE id=$1),EXISTS(SELECT 1 FROM novel_books WHERE id=$2),EXISTS(SELECT 1 FROM novel_crawl_import_task WHERE id=$3)`, item.taskID, item.bookID, item.importTaskID).Scan(&taskOK, &bookOK, &importOK); err != nil {
			return BatchResult{}, err
		}
		if !taskOK || !bookOK || !importOK {
			result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "MERGE_SOURCE_REFERENCE_NOT_FOUND", "merge source task, book, or import task was not migrated"))
			continue
		}
		sortSource, ok := mapLegacyMergeSortTimeSource(item.sortSource)
		if !ok {
			result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "INVALID_SORT_TIME_SOURCE", "merge source sort time source is unsupported"))
			continue
		}
		_, err = target.ExecContext(ctx, `INSERT INTO novel_book_merge_source(id,task_id,source_book_id,source_book_name,source_author_name,source_import_task_id,source_name,source_thread_id,source_thread_title,source_thread_url,sort_time,sort_time_source,source_order,old_publish_status,archived_publish_status,archive_time,created_at,legacy_source_key) SELECT $1,$2,$3,$4,$5,$6,i.source_name,$7,$8,$9,$10,$11,$12,$13,$14,COALESCE($15,now()),COALESCE($16,now()),$17 FROM novel_crawl_import_task i WHERE i.id=$6 ON CONFLICT(id) DO NOTHING`, item.id, item.taskID, item.bookID, strings.TrimSpace(item.bookName), strings.TrimSpace(item.authorName), item.importTaskID, strings.TrimSpace(item.threadID), strings.TrimSpace(item.threadTitle), strings.TrimSpace(item.threadURL), item.sortTime, sortSource, item.order, strings.TrimSpace(item.oldStatus), strings.TrimSpace(item.archivedStatus), item.archiveTime, item.created, key)
		if err != nil {
			return BatchResult{}, fmt.Errorf("upsert merge source %d: %w", item.id, err)
		}
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	if _, err = target.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('novel_book_merge_source','id'),GREATEST(COALESCE((SELECT max(id) FROM novel_book_merge_source),1),1),true)`); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

type NovelBookMergeChaptersStage struct{}

func (NovelBookMergeChaptersStage) Name() string { return "novel-book-merge-chapters" }

type legacyMergeChapter struct {
	id, taskID, sourceBookID, sourceChapterID  int64
	sourceNo                                   int
	sourceName                                 string
	targetBookID, targetChapterID              sql.NullInt64
	targetNo                                   sql.NullInt64
	targetName                                 sql.NullString
	contentSource                              string
	cleanID                                    sql.NullInt64
	sortTime                                   sql.NullTime
	sortSource, duplicateReason, excludeReason string
	duplicate, excluded                        int
	created                                    sql.NullTime
}

func (NovelBookMergeChaptersStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	table, err := legacyMergeTable(ctx, source, "chapter")
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
	rows, err := source.QueryContext(ctx, fmt.Sprintf(`SELECT id,task_id,source_book_id,source_chapter_id,COALESCE(source_chapter_no,0),COALESCE(source_chapter_name,''),target_book_id,target_chapter_id,target_chapter_no,target_chapter_name,content_source,clean_result_id,sort_time,COALESCE(sort_time_source,''),COALESCE(duplicate_flag,0),COALESCE(duplicate_reason,''),COALESCE(excluded,0),COALESCE(exclude_reason,''),create_time FROM %s WHERE id>? ORDER BY id LIMIT ?`, table), last, limit)
	if err != nil {
		return BatchResult{}, fmt.Errorf("query legacy merge chapters: %w", err)
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": table}}
	for rows.Next() {
		var item legacyMergeChapter
		if err := rows.Scan(&item.id, &item.taskID, &item.sourceBookID, &item.sourceChapterID, &item.sourceNo, &item.sourceName, &item.targetBookID, &item.targetChapterID, &item.targetNo, &item.targetName, &item.contentSource, &item.cleanID, &item.sortTime, &item.sortSource, &item.duplicate, &item.duplicateReason, &item.excluded, &item.excludeReason, &item.created); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(item.id, 10)
		key := "moonbook-v1:" + table + ":" + result.NextCursor
		if item.id <= 0 || item.taskID <= 0 || item.sourceBookID <= 0 || item.sourceChapterID <= 0 || item.sourceNo < 0 || !item.sortTime.Valid {
			result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "INVALID_MERGE_CHAPTER", "legacy merge chapter fields are invalid"))
			continue
		}
		if err := checkLegacyKey(ctx, target, "novel_book_merge_chapter", item.id, key); err != nil {
			result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "SOURCE_KEY_CONFLICT", err.Error()))
			continue
		}
		var taskOK, sourceChapterOK, sourceObjectOK bool
		if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_book_merge_task WHERE id=$1),EXISTS(SELECT 1 FROM novel_chapters WHERE id=$2 AND book_id=$3),EXISTS(SELECT 1 FROM novel_objects WHERE object_kind='chapter_content' AND book_id=$3 AND owner_id=$2 AND state='active')`, item.taskID, item.sourceChapterID, item.sourceBookID).Scan(&taskOK, &sourceChapterOK, &sourceObjectOK); err != nil {
			return BatchResult{}, err
		}
		if !taskOK || !sourceChapterOK || !sourceObjectOK {
			result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "MERGE_CHAPTER_SOURCE_NOT_FOUND", "merge task, source chapter, or source object was not migrated"))
			continue
		}
		var sourceObjectID int64
		if err := target.QueryRowContext(ctx, `SELECT id FROM novel_objects WHERE object_kind='chapter_content' AND book_id=$1 AND owner_id=$2 AND state='active' ORDER BY version DESC LIMIT 1`, item.sourceBookID, item.sourceChapterID).Scan(&sourceObjectID); err != nil {
			return BatchResult{}, err
		}
		var targetBook, targetChapter, targetNo, targetName, targetObject, cleanID any
		if item.excluded == 0 {
			if !item.targetBookID.Valid || !item.targetChapterID.Valid || !item.targetNo.Valid {
				result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "TARGET_CHAPTER_NOT_FOUND", "included merge chapter has no target chapter identity"))
				continue
			}
			var targetChapterOK bool
			if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_chapters WHERE id=$1 AND book_id=$2)`, item.targetChapterID.Int64, item.targetBookID.Int64).Scan(&targetChapterOK); err != nil {
				return BatchResult{}, err
			}
			if !targetChapterOK {
				result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "TARGET_CHAPTER_NOT_FOUND", "included merge target chapter was not migrated"))
				continue
			}
			var objectID int64
			if err := target.QueryRowContext(ctx, `SELECT id FROM novel_objects WHERE object_kind IN ('chapter_content','chapter_clean') AND book_id=$1 AND owner_id=$2 AND state='active' ORDER BY CASE object_kind WHEN 'chapter_clean' THEN 0 ELSE 1 END,version DESC LIMIT 1`, item.targetBookID.Int64, item.targetChapterID.Int64).Scan(&objectID); err != nil {
				result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "TARGET_OBJECT_NOT_FOUND", "included merge target chapter has no active object"))
				continue
			}
			targetBook = item.targetBookID.Int64
			targetChapter = item.targetChapterID.Int64
			targetNo = item.targetNo.Int64
			targetObject = objectID
			if item.targetName.Valid {
				targetName = item.targetName.String
			}
			if item.cleanID.Valid {
				cleanID = item.cleanID.Int64
			}
		}
		if item.contentSource != "clean_result" && item.contentSource != "original" {
			result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "INVALID_CONTENT_SOURCE", "merge chapter content source is unsupported"))
			continue
		}
		if item.contentSource == "clean_result" && !item.cleanID.Valid {
			result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "CLEAN_RESULT_NOT_FOUND", "clean_result content source has no result ID"))
			continue
		}
		if item.contentSource == "clean_result" && item.cleanID.Valid {
			var cleanOK bool
			if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_chapter_clean_result WHERE id=$1 AND active AND status='success' AND cleaned_object_id IS NOT NULL)`, item.cleanID.Int64).Scan(&cleanOK); err != nil {
				return BatchResult{}, err
			}
			if !cleanOK {
				result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "CLEAN_RESULT_NOT_FOUND", "legacy clean result was not migrated as an active successful result"))
				continue
			}
		}
		sortSource, ok := mapLegacyMergeSortTimeSource(item.sortSource)
		if !ok {
			result.Errors = append(result.Errors, mergeError(table, result.NextCursor, "INVALID_SORT_TIME_SOURCE", "merge chapter sort time source is unsupported"))
			continue
		}
		_, err = target.ExecContext(ctx, `INSERT INTO novel_book_merge_chapter(id,task_id,source_book_id,source_chapter_id,source_chapter_no,source_chapter_name,source_object_id,target_book_id,target_chapter_id,target_chapter_no,target_chapter_name,target_object_id,content_source,clean_result_id,sort_time,sort_time_source,duplicate_flag,duplicate_reason,excluded,exclude_reason,created_at,legacy_source_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,COALESCE($21,now()),$22) ON CONFLICT(id) DO NOTHING`, item.id, item.taskID, item.sourceBookID, item.sourceChapterID, item.sourceNo, strings.TrimSpace(item.sourceName), sourceObjectID, targetBook, targetChapter, targetNo, targetName, targetObject, item.contentSource, cleanID, item.sortTime, sortSource, item.duplicate != 0, strings.TrimSpace(item.duplicateReason), item.excluded != 0, strings.TrimSpace(item.excludeReason), item.created, key)
		if err != nil {
			return BatchResult{}, fmt.Errorf("upsert merge chapter %d: %w", item.id, err)
		}
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	if _, err = target.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('novel_book_merge_chapter','id'),GREATEST(COALESCE((SELECT max(id) FROM novel_book_merge_chapter),1),1),true)`); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

func legacyMergeTable(ctx context.Context, source *sql.DB, kind string) (string, error) {
	for _, name := range []string{"novel_book_merge_" + kind} {
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
func mapLegacyMergeTaskStatus(v string) (string, bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "running", "pending", "processing":
		return "failed", true, true
	case "success", "succeeded", "completed":
		return "succeeded", false, true
	case "failed", "error":
		return "failed", false, true
	default:
		return "", false, false
	}
}
func mapLegacyMergeSortTimeSource(v string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "thread_create_time", "thread_created_at":
		return "thread_created_at", true
	case "import_task_create_time", "import_task_created_at":
		return "import_task_created_at", true
	default:
		return "", false
	}
}
func mergeError(table, id, code, msg string) RecordError {
	return RecordError{SourceTable: table, SourceID: id, Code: code, Message: msg, Retryable: false}
}
