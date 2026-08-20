package legacymigrate

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// NovelCrawlCandidatesStage migrates discovered forum threads after sources
// and boards have been migrated. Import task references are intentionally
// linked by the later import-task stage.
type NovelCrawlCandidatesStage struct{}

func (NovelCrawlCandidatesStage) Name() string { return "novel-crawl-candidates" }

type legacyCrawlCandidate struct {
	id, sourceID, boardID                  int64
	sourceName, boardName                  string
	threadID, title, displayTitle          string
	threadURL, authorID                    string
	status                                 string
	importTaskID, targetBookID             sql.NullInt64
	lastPage                               sql.NullInt64
	lastFloor                              sql.NullString
	lastFollow, discover, created, updated sql.NullTime
	followCount                            int64
	followFail, remark                     string
}

func (NovelCrawlCandidatesStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	lastID, err := parseCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	exists, err := sourceTableExists(ctx, source, "novel_crawl_thread_candidate")
	if err != nil {
		return BatchResult{}, err
	}
	if !exists {
		return BatchResult{Done: true, Metadata: map[string]any{"source": "novel_crawl_thread_candidate", "skipped": "table_not_found"}}, nil
	}
	rows, err := source.QueryContext(ctx, `SELECT id,source_id,COALESCE(source_name,''),board_id,COALESCE(board_name,''),COALESCE(forum_thread_id,''),COALESCE(thread_title,''),COALESCE(display_title,''),COALESCE(thread_url,''),COALESCE(author_id,''),COALESCE(status,'pending'),import_task_id,target_book_id,last_import_page_no,last_import_floor_id,last_follow_time,COALESCE(follow_count,0),COALESCE(follow_fail_reason,''),discover_time,COALESCE(remark,''),create_time,update_time FROM novel_crawl_thread_candidate WHERE id>? ORDER BY id LIMIT ?`, lastID, limit)
	if err != nil {
		return BatchResult{}, fmt.Errorf("query legacy crawl candidates: %w", err)
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": "novel_crawl_thread_candidate"}}
	for rows.Next() {
		var item legacyCrawlCandidate
		if err := rows.Scan(&item.id, &item.sourceID, &item.sourceName, &item.boardID, &item.boardName, &item.threadID, &item.title, &item.displayTitle, &item.threadURL, &item.authorID, &item.status, &item.importTaskID, &item.targetBookID, &item.lastPage, &item.lastFloor, &item.lastFollow, &item.followCount, &item.followFail, &item.discover, &item.remark, &item.created, &item.updated); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(item.id, 10)
		key := "moonbook-v1:novel_crawl_thread_candidate:" + result.NextCursor
		if code, message := validateLegacyCandidate(item); code != "" {
			result.Errors = append(result.Errors, crawlError("novel_crawl_thread_candidate", result.NextCursor, code, message))
			continue
		}
		if err := checkLegacyKey(ctx, target, "novel_crawl_thread_candidate", item.id, key); err != nil {
			result.Errors = append(result.Errors, crawlError("novel_crawl_thread_candidate", result.NextCursor, "SOURCE_KEY_CONFLICT", err.Error()))
			continue
		}
		var sourceExists, boardExists bool
		if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_crawl_forum_source WHERE id=$1 AND legacy_source_key IS NOT NULL), EXISTS(SELECT 1 FROM novel_crawl_forum_board WHERE id=$2 AND legacy_source_key IS NOT NULL)`, item.sourceID, item.boardID).Scan(&sourceExists, &boardExists); err != nil {
			return BatchResult{}, err
		}
		if !sourceExists || !boardExists {
			result.Errors = append(result.Errors, crawlError("novel_crawl_thread_candidate", result.NextCursor, "SOURCE_OR_BOARD_NOT_FOUND", "legacy candidate references a source or board that was not migrated"))
			continue
		}
		bookID := nullableID(item.targetBookID)
		if bookID != nil {
			var bookExists bool
			if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_books WHERE id=$1 AND deleted_at IS NULL)`, *bookID).Scan(&bookExists); err != nil {
				return BatchResult{}, err
			}
			if !bookExists {
				result.Errors = append(result.Errors, crawlError("novel_crawl_thread_candidate", result.NextCursor, "TARGET_BOOK_NOT_FOUND", "target book was not migrated; candidate kept without target_book_id"))
				bookID = nil
			}
		}
		status, ok := mapLegacyCandidateStatus(item.status)
		if !ok {
			result.Errors = append(result.Errors, crawlError("novel_crawl_thread_candidate", result.NextCursor, "INVALID_CANDIDATE_STATUS", "legacy candidate status is unsupported"))
			continue
		}
		var bookValue any
		if bookID != nil {
			bookValue = *bookID
		}
		_, err = target.ExecContext(ctx, `INSERT INTO novel_crawl_thread_candidate(id,source_id,source_name,board_id,board_name,forum_thread_id,thread_title,display_title,thread_url,author_id,status,target_book_id,last_import_page_no,last_import_floor_id,last_follow_time,follow_count,follow_fail_reason,discover_time,remark,created_at,updated_at,legacy_source_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,COALESCE($18,now()),$19,COALESCE($20,now()),COALESCE($21,now()),$22) ON CONFLICT (id) DO UPDATE SET source_id=EXCLUDED.source_id,source_name=EXCLUDED.source_name,board_id=EXCLUDED.board_id,board_name=EXCLUDED.board_name,forum_thread_id=EXCLUDED.forum_thread_id,thread_title=EXCLUDED.thread_title,display_title=EXCLUDED.display_title,thread_url=EXCLUDED.thread_url,author_id=EXCLUDED.author_id,status=EXCLUDED.status,target_book_id=EXCLUDED.target_book_id,last_import_page_no=EXCLUDED.last_import_page_no,last_import_floor_id=EXCLUDED.last_import_floor_id,last_follow_time=EXCLUDED.last_follow_time,follow_count=EXCLUDED.follow_count,follow_fail_reason=EXCLUDED.follow_fail_reason,discover_time=EXCLUDED.discover_time,remark=EXCLUDED.remark,updated_at=EXCLUDED.updated_at,legacy_source_key=EXCLUDED.legacy_source_key`, item.id, item.sourceID, strings.TrimSpace(item.sourceName), item.boardID, strings.TrimSpace(item.boardName), strings.TrimSpace(item.threadID), strings.TrimSpace(item.title), strings.TrimSpace(item.displayTitle), strings.TrimSpace(item.threadURL), strings.TrimSpace(item.authorID), status, bookValue, nullableInt(item.lastPage), nullString(item.lastFloor), item.lastFollow, item.followCount, strings.TrimSpace(item.followFail), item.discover, strings.TrimSpace(item.remark), item.created, item.updated, key)
		if err != nil {
			return BatchResult{}, fmt.Errorf("upsert legacy crawl candidate %d: %w", item.id, err)
		}
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	if err := syncIdentitySequence(ctx, target, "novel_crawl_thread_candidate"); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

func validateLegacyCandidate(item legacyCrawlCandidate) (string, string) {
	if item.id <= 0 || item.sourceID <= 0 || item.boardID <= 0 {
		return "INVALID_CANDIDATE_ID", "legacy candidate IDs must be positive"
	}
	if strings.TrimSpace(item.threadID) == "" || len(item.threadID) > 128 || strings.TrimSpace(item.title) == "" || len([]rune(item.title)) > 255 || strings.TrimSpace(item.threadURL) == "" || len(item.threadURL) > 500 {
		return "INVALID_CANDIDATE_TEXT", "legacy candidate thread identity or URL is invalid"
	}
	if item.followCount < 0 {
		return "INVALID_FOLLOW_COUNT", "legacy candidate follow count must be non-negative"
	}
	return "", ""
}

func mapLegacyCandidateStatus(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pending", "importing", "imported", "skipped", "failed":
		return strings.ToLower(strings.TrimSpace(value)), true
	case "success", "succeeded":
		return "imported", true
	case "running":
		return "importing", true
	case "stopped", "cancelled":
		return "failed", true
	default:
		return "", false
	}
}

func nullableID(value sql.NullInt64) *int64 {
	if value.Valid && value.Int64 > 0 {
		v := value.Int64
		return &v
	}
	return nil
}
func nullableInt(value sql.NullInt64) any {
	if value.Valid {
		return value.Int64
	}
	return nil
}
