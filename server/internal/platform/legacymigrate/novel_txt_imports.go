package legacymigrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/txtimport"
)

type NovelTXTImportsStage struct {
	Manifest *LegacyTXTManifest
	Files    txtimport.FileStore
}

func (NovelTXTImportsStage) Name() string { return "novel-txt-imports" }

type legacyTXTImport struct {
	id, fileSize                         int64
	fileName, status, qualityStatus      string
	qualitySummary, failReason, operator string
	targetBookID                         sql.NullInt64
	total, imported, empty, duplicate    int64
	start, end, created, updated         sql.NullTime
}

func (stage NovelTXTImportsStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	databaseFactsOnly := stage.Manifest == nil || stage.Files == nil
	lastID, err := parseCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	exists, err := sourceTableExists(ctx, source, "novel_txt_import_task")
	if err != nil {
		return BatchResult{}, err
	}
	if !exists {
		return BatchResult{Done: true, Metadata: map[string]any{"source": "novel_txt_import_task", "skipped": "table_not_found"}}, nil
	}
	rows, err := source.QueryContext(ctx, `SELECT id,COALESCE(file_name,''),COALESCE(file_size_bytes,0),target_book_id,COALESCE(status,'pending'),COALESCE(quality_status,'pending'),COALESCE(charset_detect_summary,''),COALESCE(total_chapter_count,0),COALESCE(imported_chapter_count,0),COALESCE(empty_chapter_count,0),COALESCE(duplicate_chapter_count,0),COALESCE(fail_reason,''),COALESCE(operator_name,''),start_time,end_time,create_time,update_time FROM novel_txt_import_task WHERE id>? ORDER BY id LIMIT ?`, lastID, limit)
	if err != nil {
		return BatchResult{}, fmt.Errorf("query legacy TXT imports: %w", err)
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": "novel_txt_import_task", "objectCount": 0, "objectBytes": int64(0)}}
	for rows.Next() {
		var item legacyTXTImport
		if err := rows.Scan(&item.id, &item.fileName, &item.fileSize, &item.targetBookID, &item.status, &item.qualityStatus, &item.qualitySummary, &item.total, &item.imported, &item.empty, &item.duplicate, &item.failReason, &item.operator, &item.start, &item.end, &item.created, &item.updated); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(item.id, 10)
		if code, message := validateLegacyTXTImport(item); code != "" {
			result.Errors = append(result.Errors, txtError(result.NextCursor, code, message))
			continue
		}
		key := "moonbook-v1:novel_txt_import_task:" + result.NextCursor
		if err := checkLegacyKey(ctx, target, "novel_txt_import_task", item.id, key); err != nil {
			result.Errors = append(result.Errors, txtError(result.NextCursor, "SOURCE_KEY_CONFLICT", err.Error()))
			continue
		}
		bookArg := nullableID(item.targetBookID)
		if bookArg != nil {
			var found bool
			if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_books WHERE id=$1 AND deleted_at IS NULL)`, *bookArg).Scan(&found); err != nil {
				return BatchResult{}, err
			}
			if !found {
				result.Errors = append(result.Errors, txtError(result.NextCursor, "TARGET_BOOK_NOT_FOUND", "legacy TXT target book was not migrated"))
				bookArg = nil
			}
		}
		var targetBook any
		if bookArg != nil {
			targetBook = *bookArg
		}
		status, interrupted, ok := mapLegacyTXTStatus(item.status)
		if !ok {
			result.Errors = append(result.Errors, txtError(result.NextCursor, "INVALID_TXT_STATUS", "legacy TXT task status is unsupported"))
			continue
		}
		quality, ok := mapLegacyQualityStatus(item.qualityStatus)
		if !ok {
			result.Errors = append(result.Errors, txtError(result.NextCursor, "INVALID_QUALITY_STATUS", "legacy TXT quality status is unsupported"))
			continue
		}
		if interrupted {
			status = "failed"
			if item.failReason == "" {
				item.failReason = "legacy TXT import was interrupted during cutover"
			}
		}
		var objectKey, hashText, objectSize any
		if !databaseFactsOnly {
			path, err := stage.Manifest.Resolve(item.id)
			if err != nil {
				result.Errors = append(result.Errors, txtError(result.NextCursor, "TXT_FILE_UNAVAILABLE", "legacy TXT file is missing from or invalid in the migration manifest"))
				continue
			}
			data, err := readLegacyTXT(path)
			if err != nil || (item.fileSize > 0 && item.fileSize != int64(len(data))) {
				result.Errors = append(result.Errors, txtError(result.NextCursor, "TXT_FILE_READ_FAILED", "legacy TXT file cannot be read or does not match the database snapshot"))
				continue
			}
			hash := sha256.Sum256(data)
			hashValue := hex.EncodeToString(hash[:])
			keyValue := fmt.Sprintf("imports/txt/legacy/%d/%s.txt", item.id, hashValue)
			meta, err := stage.Files.Put(ctx, keyValue, data, "text/plain; charset=utf-8")
			if err != nil || meta.ByteSize != int64(len(data)) || !strings.EqualFold(meta.SHA256, hashValue) {
				result.Errors = append(result.Errors, txtError(result.NextCursor, "TXT_OBJECT_UPLOAD_FAILED", "legacy TXT upload or size/SHA-256 verification failed"))
				continue
			}
			objectKey, hashText, objectSize = meta.Key, strings.ToLower(meta.SHA256), meta.ByteSize
		}
		payload, _ := json.Marshal(map[string]string{"txtImportTaskId": result.NextCursor})
		jobStatus := status
		var jobID int64
		if err := target.QueryRowContext(ctx, `INSERT INTO platform_jobs(module,job_type,idempotency_key,status,payload,max_attempts,finished_at,last_error_code,last_error_message) VALUES('novel','txt_import',$1,$2,$3,3,CASE WHEN $2 IN ('succeeded','failed','cancelled') THEN now() END,CASE WHEN $2='failed' THEN 'LEGACY_INTERRUPTED' END,CASE WHEN $2='failed' THEN 'legacy TXT task requires operator review' END) ON CONFLICT(module,job_type,idempotency_key) DO UPDATE SET status=EXCLUDED.status,payload=EXCLUDED.payload,finished_at=EXCLUDED.finished_at,last_error_code=EXCLUDED.last_error_code,last_error_message=EXCLUDED.last_error_message RETURNING id`, "txt-import:"+result.NextCursor, jobStatus, payload).Scan(&jobID); err != nil {
			return BatchResult{}, err
		}
		_, err = target.ExecContext(ctx, `INSERT INTO novel_txt_import_task(id,target_book_id,platform_job_id,original_filename,object_key,object_sha256,object_byte_size,status,quality_status,quality_summary,total_chapter_count,imported_chapter_count,empty_chapter_count,duplicate_chapter_count,fail_reason,operator_name,attempt_count,max_attempts,start_time,end_time,created_at,updated_at,legacy_source_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,0,3,$17,$18,COALESCE($19,now()),COALESCE($20,now()),$21) ON CONFLICT(id) DO UPDATE SET target_book_id=EXCLUDED.target_book_id,platform_job_id=EXCLUDED.platform_job_id,original_filename=EXCLUDED.original_filename,object_key=EXCLUDED.object_key,object_sha256=EXCLUDED.object_sha256,object_byte_size=EXCLUDED.object_byte_size,status=EXCLUDED.status,quality_status=EXCLUDED.quality_status,quality_summary=EXCLUDED.quality_summary,total_chapter_count=EXCLUDED.total_chapter_count,imported_chapter_count=EXCLUDED.imported_chapter_count,empty_chapter_count=EXCLUDED.empty_chapter_count,duplicate_chapter_count=EXCLUDED.duplicate_chapter_count,fail_reason=EXCLUDED.fail_reason,operator_name=EXCLUDED.operator_name,start_time=EXCLUDED.start_time,end_time=EXCLUDED.end_time,updated_at=EXCLUDED.updated_at,legacy_source_key=EXCLUDED.legacy_source_key`, item.id, targetBook, jobID, filepath.Base(item.fileName), objectKey, hashText, objectSize, status, quality, strings.TrimSpace(item.qualitySummary), item.total, item.imported, item.empty, item.duplicate, strings.TrimSpace(item.failReason), strings.TrimSpace(item.operator), item.start, item.end, item.created, item.updated, key)
		if err != nil {
			return BatchResult{}, fmt.Errorf("upsert legacy TXT import %d: %w", item.id, err)
		}
		if objectSize != nil {
			result.Metadata["objectCount"] = result.Metadata["objectCount"].(int) + 1
			result.Metadata["objectBytes"] = result.Metadata["objectBytes"].(int64) + objectSize.(int64)
		}
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	if err := syncIdentitySequence(ctx, target, "novel_txt_import_task"); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

func readLegacyTXT(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, txtimport.MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) == 0 {
		return nil, errors.New("legacy TXT file is empty")
	}
	if int64(len(data)) > txtimport.MaxFileBytes {
		return nil, errors.New("legacy TXT file exceeds the target 16 MiB limit")
	}
	return data, nil
}
func validateLegacyTXTImport(item legacyTXTImport) (string, string) {
	if item.id <= 0 || strings.TrimSpace(item.fileName) == "" {
		return "INVALID_TXT_IDENTITY", "legacy TXT task ID and filename are required"
	}
	if item.fileSize < 0 || item.total < 0 || item.imported < 0 || item.empty < 0 || item.duplicate < 0 {
		return "INVALID_TXT_COUNT", "legacy TXT size and counters must be non-negative"
	}
	return "", ""
}
func mapLegacyTXTStatus(value string) (string, bool, bool) {
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
func txtError(id, code, message string) RecordError {
	return RecordError{SourceTable: "novel_txt_import_task", SourceID: id, Code: code, Message: message, Retryable: code == "TXT_OBJECT_UPLOAD_FAILED"}
}
