package legacymigrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// NovelCrawlSourcesStage migrates forum configuration without copying cookies.
type NovelCrawlSourcesStage struct{}

func (NovelCrawlSourcesStage) Name() string { return "novel-crawl-sources" }

func (NovelCrawlSourcesStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	lastID, err := parseCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	for _, table := range []string{"novel_crawl_forum_source", "novel_crawl_forum_board"} {
		exists, err := sourceTableExists(ctx, source, table)
		if err != nil {
			return BatchResult{}, err
		}
		if !exists {
			return BatchResult{Done: true, Metadata: map[string]any{"source": table, "skipped": "table_not_found"}}, nil
		}
	}
	rows, err := source.QueryContext(ctx, `SELECT id,source_name,base_url,COALESCE(request_charset,'UTF-8'),COALESCE(cookie_text,''),COALESCE(user_agent,''),COALESCE(request_interval_ms,1000),last_request_time,COALESCE(enabled,1),COALESCE(sort_order,0),COALESCE(remark,''),create_time,update_time FROM novel_crawl_forum_source WHERE id>? ORDER BY id LIMIT ?`, lastID, limit)
	if err != nil {
		return BatchResult{}, fmt.Errorf("query legacy forum sources: %w", err)
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": "novel_crawl_forum_source+novel_crawl_forum_board", "cookiesOmitted": 0}}
	for rows.Next() {
		var id, interval, enabled, sortOrder int64
		var name, baseURL, charset, cookie, userAgent, remark string
		var lastRequest, created, updated sql.NullTime
		if err := rows.Scan(&id, &name, &baseURL, &charset, &cookie, &userAgent, &interval, &lastRequest, &enabled, &sortOrder, &remark, &created, &updated); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(id, 10)
		key := "moonbook-v1:novel_crawl_forum_source:" + result.NextCursor
		if err := checkLegacyKey(ctx, target, "novel_crawl_forum_source", id, key); err != nil {
			result.Errors = append(result.Errors, crawlError("novel_crawl_forum_source", result.NextCursor, "SOURCE_KEY_CONFLICT", err.Error()))
			continue
		}
		_, err = target.ExecContext(ctx, `INSERT INTO novel_crawl_forum_source(id,source_name,base_url,request_charset,cookie_text,user_agent,request_interval_ms,last_request_time,enabled,sort_order,remark,created_at,updated_at,legacy_source_key) VALUES($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),$7,$8,$9<>0,$10,$11,COALESCE($12,now()),COALESCE($13,now()),$14) ON CONFLICT (id) DO UPDATE SET source_name=EXCLUDED.source_name,base_url=EXCLUDED.base_url,request_charset=EXCLUDED.request_charset,user_agent=EXCLUDED.user_agent,request_interval_ms=EXCLUDED.request_interval_ms,last_request_time=EXCLUDED.last_request_time,enabled=EXCLUDED.enabled,sort_order=EXCLUDED.sort_order,remark=EXCLUDED.remark,updated_at=EXCLUDED.updated_at,legacy_source_key=EXCLUDED.legacy_source_key`, id, strings.TrimSpace(name), strings.TrimSpace(baseURL), strings.TrimSpace(charset), "", strings.TrimSpace(userAgent), interval, lastRequest, enabled, sortOrder, strings.TrimSpace(remark), created, updated, key)
		if err != nil {
			return BatchResult{}, fmt.Errorf("upsert legacy forum source %d: %w", id, err)
		}
		if cookie != "" {
			result.Errors = append(result.Errors, crawlError("novel_crawl_forum_source", result.NextCursor, "COOKIE_SECRET_REQUIRED", "legacy forum cookie omitted; provision a deployment secret before enabling this source"))
			result.Metadata["cookiesOmitted"] = result.Metadata["cookiesOmitted"].(int) + 1
		}
		if err := migrateForumBoards(ctx, source, target, id, &result); err != nil {
			return BatchResult{}, err
		}
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

func migrateForumBoards(ctx context.Context, source *sql.DB, target *sql.Tx, sourceID int64, result *BatchResult) error {
	boards, err := source.QueryContext(ctx, `SELECT id,source_id,source_name,board_name,board_url,board_url_template,last_cursor,COALESCE(auto_follow_enabled,0),COALESCE(follow_interval_minutes,60),COALESCE(follow_import_limit,20),last_follow_time,COALESCE(enabled,1),COALESCE(sort_order,0),COALESCE(remark,''),create_time,update_time FROM novel_crawl_forum_board WHERE source_id=? ORDER BY id`, sourceID)
	if err != nil {
		return err
	}
	defer boards.Close()
	for boards.Next() {
		var id, sid, followEnabled, followInterval, followLimit, enabled, sortOrder int64
		var sourceName, boardName, boardURL string
		var template, cursor, remark sql.NullString
		var followTime, created, updated sql.NullTime
		if err := boards.Scan(&id, &sid, &sourceName, &boardName, &boardURL, &template, &cursor, &followEnabled, &followInterval, &followLimit, &followTime, &enabled, &sortOrder, &remark, &created, &updated); err != nil {
			return err
		}
		key := "moonbook-v1:novel_crawl_forum_board:" + strconv.FormatInt(id, 10)
		if err := checkLegacyKey(ctx, target, "novel_crawl_forum_board", id, key); err != nil {
			result.Errors = append(result.Errors, crawlError("novel_crawl_forum_board", strconv.FormatInt(id, 10), "SOURCE_KEY_CONFLICT", err.Error()))
			continue
		}
		_, err = target.ExecContext(ctx, `INSERT INTO novel_crawl_forum_board(id,source_id,source_name,board_name,board_url,board_url_template,last_cursor,auto_follow_enabled,follow_interval_minutes,follow_import_limit,last_follow_time,enabled,sort_order,remark,created_at,updated_at,legacy_source_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8<>0,$9,$10,$11,$12<>0,$13,$14,COALESCE($15,now()),COALESCE($16,now()),$17) ON CONFLICT (id) DO UPDATE SET source_id=EXCLUDED.source_id,source_name=EXCLUDED.source_name,board_name=EXCLUDED.board_name,board_url=EXCLUDED.board_url,board_url_template=EXCLUDED.board_url_template,last_cursor=EXCLUDED.last_cursor,auto_follow_enabled=EXCLUDED.auto_follow_enabled,follow_interval_minutes=EXCLUDED.follow_interval_minutes,follow_import_limit=EXCLUDED.follow_import_limit,last_follow_time=EXCLUDED.last_follow_time,enabled=EXCLUDED.enabled,sort_order=EXCLUDED.sort_order,remark=EXCLUDED.remark,updated_at=EXCLUDED.updated_at,legacy_source_key=EXCLUDED.legacy_source_key`, id, sid, strings.TrimSpace(sourceName), strings.TrimSpace(boardName), strings.TrimSpace(boardURL), nullString(template), nullString(cursor), followEnabled, followInterval, followLimit, followTime, enabled, sortOrder, nullString(remark), created, updated, key)
		if err != nil {
			return fmt.Errorf("upsert legacy forum board %d: %w", id, err)
		}
	}
	return boards.Err()
}

func checkLegacyKey(ctx context.Context, target *sql.Tx, table string, id int64, key string) error {
	var existing string
	err := target.QueryRowContext(ctx, "SELECT COALESCE(legacy_source_key,'') FROM "+table+" WHERE id=$1", id).Scan(&existing)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && existing != "" && existing != key {
		return fmt.Errorf("target %s id %d has a different legacy source key", table, id)
	}
	return nil
}

func crawlError(table, id, code, message string) RecordError {
	return RecordError{SourceTable: table, SourceID: id, Code: code, Message: message, Retryable: false}
}
func nullString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}
