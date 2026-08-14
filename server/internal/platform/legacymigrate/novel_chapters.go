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
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
)

const legacyChapterMaxBytes = (16 << 20) - 1

type NovelChaptersStage struct {
	Objects *objectstore.Service
}

func (NovelChaptersStage) Name() string { return "novel-chapters" }

type legacyChapter struct {
	id, bookID                   int64
	chapterNo, wordCount, isVIP  int
	bookPrice                    int64
	chapterName, storageType     string
	chapterStatus, aiCleanStatus string
	content                      sql.NullString
	createdAt, updatedAt         sql.NullTime
}

func (stage NovelChaptersStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	if stage.Objects == nil {
		return BatchResult{}, errors.New("chapter migration requires object storage")
	}
	lastID, err := parseCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	rows, err := source.QueryContext(ctx, `SELECT c.id,c.book_id,c.chapter_no,c.chapter_name,c.word_count,c.is_vip,c.book_price,c.storage_type,c.chapter_status,c.ai_clean_status,c.create_time,c.update_time,cc.content
		FROM novel_chapter c LEFT JOIN novel_chapter_content cc ON cc.chapter_id=c.id
		WHERE c.id>? ORDER BY c.id LIMIT ?`, lastID, limit)
	if err != nil {
		return BatchResult{}, fmt.Errorf("query legacy novel chapters: %w", err)
	}
	items := make([]legacyChapter, 0, limit)
	for rows.Next() {
		var item legacyChapter
		if err := rows.Scan(&item.id, &item.bookID, &item.chapterNo, &item.chapterName, &item.wordCount, &item.isVIP, &item.bookPrice, &item.storageType, &item.chapterStatus, &item.aiCleanStatus, &item.createdAt, &item.updatedAt, &item.content); err != nil {
			rows.Close()
			return BatchResult{}, err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return BatchResult{}, err
	}
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": "novel_chapter+novel_chapter_content"}}
	for _, item := range items {
		result.Processed++
		result.NextCursor = strconv.FormatInt(item.id, 10)
		recordError := func(code, message string, retryable bool) {
			result.Errors = append(result.Errors, RecordError{SourceTable: "novel_chapter", SourceID: result.NextCursor, Code: code, Message: message, Retryable: retryable})
		}
		if code, message := validateLegacyChapter(item); code != "" {
			recordError(code, message, false)
			continue
		}
		var existingSource string
		err := target.QueryRowContext(ctx, `SELECT source_type FROM novel_chapters WHERE id=$1`, item.id).Scan(&existingSource)
		if err == nil {
			if existingSource != "legacy" {
				recordError("CHAPTER_ID_CONFLICT", "target chapter ID already belongs to a non-legacy record", false)
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return BatchResult{}, fmt.Errorf("check target chapter %d: %w", item.id, err)
		}
		var bookExists bool
		if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_books WHERE id=$1 AND deleted_at IS NULL)`, item.bookID).Scan(&bookExists); err != nil {
			return BatchResult{}, err
		}
		if !bookExists {
			recordError("BOOK_NOT_FOUND", "legacy chapter references a missing target book", false)
			continue
		}
		text := item.content.String
		actualWords := countLegacyChapterWords(text)
		fingerprint := legacyChapterFingerprint(item, text)
		object, err := stage.Objects.UploadVerifiedWithOptions(ctx,
			objectstore.Target{Kind: objectstore.KindChapterContent, BookID: item.bookID, OwnerID: item.id},
			[]byte(text), "text/plain; charset=utf-8", objectstore.UploadOptions{Source: "legacy", SourceFingerprint: fingerprint})
		if err != nil {
			recordError("CHAPTER_CONTENT_UPLOAD_FAILED", "legacy chapter content upload or verification failed", true)
			continue
		}
		if object.State == objectstore.StateActive {
			continue
		}
		chapterStatus, _ := mapLegacyChapterStatus(item.chapterStatus)
		cleanStatus, _ := mapLegacyCleanStatus(item.aiCleanStatus)
		err = stage.Objects.ActivateWithTx(ctx, object.ID, func(tx *sql.Tx, target objectstore.Target) error {
			if target.BookID != item.bookID || target.OwnerID != item.id {
				return errors.New("legacy chapter object target mismatch")
			}
			var exists bool
			if err := tx.QueryRowContext(ctx, `SELECT true FROM novel_books WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, item.bookID).Scan(&exists); err != nil {
				return err
			}
			createdAt := legacyTimeOr(item.createdAt, time.Now().UTC())
			updatedAt := legacyTimeOr(item.updatedAt, createdAt)
			_, err := tx.ExecContext(ctx, `INSERT INTO novel_chapters
				(id,book_id,chapter_no,chapter_name,word_count,is_vip,book_price_coin,chapter_status,ai_clean_status,source_type,created_at,updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'legacy',$10,$11)`, item.id, item.bookID, item.chapterNo, strings.TrimSpace(item.chapterName), actualWords, item.isVIP == 1, item.bookPrice, chapterStatus, cleanStatus, createdAt, updatedAt)
			if err != nil {
				return err
			}
			return refreshLegacyBookStats(ctx, tx, item.bookID)
		})
		if err != nil {
			recordError("CHAPTER_ACTIVATE_FAILED", "legacy chapter metadata and content activation failed", true)
			continue
		}
		if actualWords != item.wordCount {
			recordError("WORD_COUNT_RECALCULATED", "legacy word count differed from normalized content and was recalculated", false)
		}
	}
	if _, err := target.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('novel_chapters','id'),GREATEST(COALESCE((SELECT max(id) FROM novel_chapters),1),1),true)`); err != nil {
		return BatchResult{}, fmt.Errorf("advance novel chapter identity: %w", err)
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

func validateLegacyChapter(item legacyChapter) (string, string) {
	if item.id <= 0 || item.bookID <= 0 {
		return "INVALID_CHAPTER_ID", "legacy chapter and book IDs must be positive"
	}
	name := strings.TrimSpace(item.chapterName)
	if name == "" || len([]rune(name)) > 255 {
		return "INVALID_CHAPTER_NAME", "legacy chapter name is blank or exceeds 255 characters"
	}
	if item.chapterNo < 0 || item.wordCount < 0 || item.bookPrice < 0 {
		return "INVALID_CHAPTER_NUMBER", "legacy chapter number, word count, and price must be non-negative"
	}
	if item.isVIP != 0 && item.isVIP != 1 {
		return "INVALID_CHAPTER_VIP", "legacy chapter is_vip must be 0 or 1"
	}
	if !item.content.Valid {
		return "CHAPTER_CONTENT_NOT_FOUND", "legacy chapter has no content row"
	}
	if !utf8.ValidString(item.content.String) || len(item.content.String) > legacyChapterMaxBytes {
		return "INVALID_CHAPTER_CONTENT", "legacy chapter content is not valid UTF-8 or exceeds MEDIUMTEXT size"
	}
	if _, ok := mapLegacyChapterStatus(item.chapterStatus); !ok {
		return "INVALID_CHAPTER_STATUS", "legacy chapter status is unsupported"
	}
	if _, ok := mapLegacyCleanStatus(item.aiCleanStatus); !ok {
		return "INVALID_CHAPTER_CLEAN_STATUS", "legacy chapter AI clean status is unsupported"
	}
	return "", ""
}

func mapLegacyChapterStatus(value string) (string, bool) {
	switch strings.TrimSpace(value) {
	case "0":
		return "enabled", true
	case "1":
		return "disabled", true
	default:
		return "", false
	}
}

func mapLegacyCleanStatus(value string) (string, bool) {
	values := map[string]string{"0": "pending", "1": "cleaning", "2": "cleaned", "3": "discarded", "4": "failed", "5": "expired", "6": "skipped"}
	result, ok := values[strings.TrimSpace(value)]
	return result, ok
}

func countLegacyChapterWords(content string) int {
	count := 0
	for _, value := range content {
		if !unicode.IsSpace(value) {
			count++
		}
	}
	return count
}

func legacyChapterFingerprint(item legacyChapter, content string) string {
	contentDigest := sha256.Sum256([]byte(content))
	updatedAt := ""
	if item.updatedAt.Valid {
		updatedAt = item.updatedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	identity := fmt.Sprintf("novel_chapter:%d:%s:%s", item.id, updatedAt, hex.EncodeToString(contentDigest[:]))
	digest := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(digest[:])
}

func legacyTimeOr(value sql.NullTime, fallback time.Time) time.Time {
	if value.Valid {
		return value.Time
	}
	return fallback
}

func refreshLegacyBookStats(ctx context.Context, tx *sql.Tx, bookID int64) error {
	_, err := tx.ExecContext(ctx, `UPDATE novel_books b SET
		word_count=COALESCE((SELECT sum(c.word_count)::integer FROM novel_chapters c WHERE c.book_id=b.id AND c.deleted_at IS NULL AND c.chapter_status='enabled'),0),
		last_chapter_id=(SELECT c.id FROM novel_chapters c WHERE c.book_id=b.id AND c.deleted_at IS NULL AND c.chapter_status='enabled' ORDER BY c.chapter_no DESC,c.id DESC LIMIT 1),
		last_chapter_name=(SELECT c.chapter_name FROM novel_chapters c WHERE c.book_id=b.id AND c.deleted_at IS NULL AND c.chapter_status='enabled' ORDER BY c.chapter_no DESC,c.id DESC LIMIT 1),
		last_chapter_updated_at=(SELECT c.updated_at FROM novel_chapters c WHERE c.book_id=b.id AND c.deleted_at IS NULL AND c.chapter_status='enabled' ORDER BY c.chapter_no DESC,c.id DESC LIMIT 1),updated_at=now()
		WHERE b.id=$1 AND b.deleted_at IS NULL`, bookID)
	return err
}
