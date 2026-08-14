package legacymigrate

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// ReaderCommerceStage imports the append-only reader facts and legacy
// commerce facts in a deterministic table order. Each table has its own id
// cursor, so a failed batch can be resumed without re-reading earlier tables.
type ReaderCommerceStage struct{}

func (ReaderCommerceStage) Name() string { return "reader-commerce" }

var readerCommerceTables = []string{"reader_invite_code", "reader_invite_relation", "reader_product", "reader_membership_grant", "reader_entitlement", "reader_book_like", "reader_bookshelf", "reader_read_history", "reader_reading_history", "reader_reading_preference", "user_bookshelf", "user_read_history", "user_feedback", "reader_feedback"}

func (ReaderCommerceStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	table, last, err := commerceCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	for i := indexOf(readerCommerceTables, table); i < len(readerCommerceTables); i++ {
		table = readerCommerceTables[i]
		exists, e := sourceTableExists(ctx, source, table)
		if e != nil {
			return BatchResult{}, e
		}
		if !exists {
			continue
		}
		return runCommerceTable(ctx, source, target, table, last, limit)
	}
	return BatchResult{Done: true, NextCursor: "reader_feedback:0", Metadata: map[string]any{"source": "reader commerce", "skipped": "table_not_found"}}, nil
}

func indexOf(values []string, value string) int {
	for i, v := range values {
		if v == value {
			return i
		}
	}
	return 0
}
func commerceCursor(raw string) (string, int64, error) {
	if strings.TrimSpace(raw) == "" {
		return readerCommerceTables[0], 0, nil
	}
	return stageCursor(raw, readerCommerceTables[0])
}

func runCommerceTable(ctx context.Context, source *sql.DB, target *sql.Tx, table string, last int64, limit int) (BatchResult, error) {
	q := map[string]string{
		"reader_invite_code":        `SELECT id,code,COALESCE(inviter_reader_id,0),COALESCE(status,'enabled'),max_use_count,used_count,expire_time,COALESCE(remark,''),create_time,update_time FROM reader_invite_code WHERE id>? ORDER BY id LIMIT ?`,
		"reader_invite_relation":    `SELECT id,inviter_reader_id,invitee_reader_id,invite_code,COALESCE(status,'active'),create_time FROM reader_invite_relation WHERE id>? ORDER BY id LIMIT ?`,
		"reader_product":            `SELECT id,product_type,COALESCE(target_id,0),product_name,price_coin,allow_bonus_coin,duration_days,sale_status,sort_order,create_time,update_time FROM reader_product WHERE id>? ORDER BY id LIMIT ?`,
		"reader_membership_grant":   `SELECT id,reader_id,grant_type,create_time,after_expire_time,after_permanent,status FROM reader_membership_grant WHERE id>? ORDER BY id LIMIT ?`,
		"reader_entitlement":        `SELECT id,reader_id,entitlement_type,target_id,start_time,expire_time,status,create_time,update_time FROM reader_entitlement WHERE id>? ORDER BY id LIMIT ?`,
		"reader_book_like":          `SELECT id,reader_id,book_id,create_time FROM reader_book_like WHERE id>? ORDER BY id LIMIT ?`,
		"reader_bookshelf":          `SELECT id,reader_id,book_id,last_chapter_id,last_read_time,create_time,update_time FROM reader_bookshelf WHERE id>? ORDER BY id LIMIT ?`,
		"user_bookshelf":            `SELECT id,user_id,book_id,pre_content_id,create_time,update_time FROM user_bookshelf WHERE id>? ORDER BY id LIMIT ?`,
		"user_read_history":         `SELECT id,user_id,book_id,pre_content_id,create_time,update_time FROM user_read_history WHERE id>? ORDER BY id LIMIT ?`,
		"reader_reading_history":    `SELECT id,reader_id,book_id,chapter_id,chapter_no,position_type,position_value,progress_percent,last_read_time,create_time,update_time FROM reader_reading_history WHERE id>? ORDER BY id LIMIT ?`,
		"reader_read_history":       `SELECT id,reader_id,book_id,chapter_id,chapter_no,position_type,position_value,progress_percent,last_read_time,create_time,update_time FROM reader_read_history WHERE id>? ORDER BY id LIMIT ?`,
		"reader_reading_preference": `SELECT id,reader_id,font_size,line_height,theme,reading_mode,create_time,update_time FROM reader_reading_preference WHERE id>? ORDER BY id LIMIT ?`,
		"user_feedback":             `SELECT id,COALESCE(user_id,0),COALESCE(content,''),create_time FROM user_feedback WHERE id>? ORDER BY id LIMIT ?`,
		"reader_feedback":           `SELECT id,reader_id,book_id,chapter_id,content,status,reply,replied_at,created_at,updated_at FROM reader_feedback WHERE id>? ORDER BY id LIMIT ?`,
	}
	query, ok := q[table]
	if !ok {
		next := nextCommerceCursor(table)
		return BatchResult{Done: next == "", NextCursor: next, Metadata: map[string]any{"source": table, "skipped": "unsupported_legacy_shape"}}, nil
	}
	rows, err := source.QueryContext(ctx, query, last, limit)
	if err != nil {
		return BatchResult{}, err
	}
	defer rows.Close()
	result := BatchResult{NextCursor: fmt.Sprintf("%s:%d", table, last), Metadata: map[string]any{"source": table}}
	for rows.Next() {
		var id int64
		var a, b, c int64
		var s1, s2, s3 string
		var n1 sql.NullInt64
		var t1, t2, t3 sql.NullTime
		var flag bool
		var scanErr error
		switch table {
		case "reader_invite_code":
			scanErr = rows.Scan(&id, &s1, &a, &s2, &n1, &b, &t1, &s3, &t2, &t3)
			if scanErr == nil {
				_, scanErr = target.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status,max_use_count,used_count,expires_at,remark,created_at,updated_at) VALUES($1,$2,NULLIF($3,0),$4,$5,$6,$7,$8,COALESCE($9,now()),COALESCE($10,now())) ON CONFLICT(id) DO UPDATE SET code=EXCLUDED.code,status=EXCLUDED.status,used_count=EXCLUDED.used_count,updated_at=EXCLUDED.updated_at`, id, s1, a, s2, n1, b, nullableLegacyTime(t1), s3, nullableLegacyTime(t2), nullableLegacyTime(t3))
			}
		case "reader_invite_relation":
			scanErr = rows.Scan(&id, &a, &b, &s1, &s2, &t1)
			if scanErr == nil {
				var codeID int64
				scanErr = target.QueryRowContext(ctx, `SELECT id FROM reader_invite_codes WHERE code=$1`, s1).Scan(&codeID)
				if scanErr == nil {
					_, scanErr = target.ExecContext(ctx, `INSERT INTO reader_invite_relations(id,inviter_reader_id,invitee_reader_id,invite_code_id,status,created_at) VALUES($1,$2,$3,$4,$5,COALESCE($6,now())) ON CONFLICT(invitee_reader_id) DO NOTHING`, id, a, b, codeID, s2, nullableLegacyTime(t1))
				}
			}
		case "reader_product":
			scanErr = rows.Scan(&id, &s1, &a, &s2, &b, &flag, &n1, &s3, &c, &t1, &t2)
			if scanErr == nil {
				_, scanErr = target.ExecContext(ctx, `INSERT INTO commerce_products(id,product_type,target_id,product_name,price_coin,allow_bonus_coin,duration_days,sale_status,sort_order,source_type,source_ref,created_at,updated_at) VALUES($1,$2,NULLIF($3,0),$4,$5,$6,$7,$8,$9,'legacy',$1,COALESCE($10,now()),COALESCE($11,now())) ON CONFLICT(id) DO UPDATE SET product_name=EXCLUDED.product_name,price_coin=EXCLUDED.price_coin,sale_status=EXCLUDED.sale_status,updated_at=EXCLUDED.updated_at`, id, s1, a, s2, b, flag, n1, s3, c, nullableLegacyTime(t1), nullableLegacyTime(t2))
			}
		case "reader_book_like":
			scanErr = rows.Scan(&id, &a, &b, &t1)
			if scanErr == nil {
				_, scanErr = target.ExecContext(ctx, `INSERT INTO reader_book_likes(id,reader_id,book_id,created_at) VALUES($1,$2,$3,COALESCE($4,now())) ON CONFLICT(reader_id,book_id) DO NOTHING`, id, a, b, nullableLegacyTime(t1))
			}
		case "reader_bookshelf":
			scanErr = rows.Scan(&id, &a, &b, &n1, &t1, &t2, &t3)
			if scanErr == nil {
				_, scanErr = target.ExecContext(ctx, `INSERT INTO reader_bookshelf_entries(id,reader_id,book_id,last_chapter_id,last_read_at,created_at,updated_at) VALUES($1,$2,$3,NULLIF($4,0),$5,COALESCE($6,now()),COALESCE($7,now())) ON CONFLICT(reader_id,book_id) DO UPDATE SET last_chapter_id=EXCLUDED.last_chapter_id,last_read_at=EXCLUDED.last_read_at,updated_at=EXCLUDED.updated_at`, id, a, b, n1.Int64, nullableLegacyTime(t1), nullableLegacyTime(t2), nullableLegacyTime(t3))
			}
		case "user_bookshelf":
			scanErr = rows.Scan(&id, &a, &b, &n1, &t1, &t2)
			if scanErr == nil {
				_, scanErr = target.ExecContext(ctx, `INSERT INTO reader_bookshelf_entries(id,reader_id,book_id,last_chapter_id,last_read_at,created_at,updated_at) VALUES($1,$2,$3,NULLIF($4,0),$5,COALESCE($6,now()),COALESCE($6,now())) ON CONFLICT(reader_id,book_id) DO NOTHING`, id, a, b, n1.Int64, nullableLegacyTime(t2), nullableLegacyTime(t1))
			}
		case "reader_feedback":
			scanErr = rows.Scan(&id, &a, &n1, &n1, &s1, &s2, &s3, &t1, &t2, &t3)
			if scanErr == nil {
				if s2 == "pending" {
					s3 = ""
				}
				_, scanErr = target.ExecContext(ctx, `INSERT INTO reader_feedback(id,reader_id,content,status,reply,created_at,updated_at) VALUES($1,$2,$3,$4,$5,COALESCE($6,now()),COALESCE($7,now())) ON CONFLICT(id) DO NOTHING`, id, a, s1, s2, s3, nullableLegacyTime(t2), nullableLegacyTime(t3))
			}
		case "user_feedback":
			scanErr = rows.Scan(&id, &a, &s1, &t1)
			if scanErr == nil {
				_, scanErr = target.ExecContext(ctx, `INSERT INTO reader_feedback(id,reader_id,content,status,reply,created_at,updated_at) VALUES($1,NULLIF($2,0),$3,'pending','',COALESCE($4,now()),COALESCE($4,now())) ON CONFLICT(id) DO NOTHING`, id, a, s1, nullableLegacyTime(t1))
			}
		case "reader_reading_preference":
			scanErr = rows.Scan(&id, &a, &n1, &s1, &s2, &s3, &t1, &t2)
			if scanErr == nil {
				_, scanErr = target.ExecContext(ctx, `INSERT INTO reader_reading_preferences(id,reader_id,font_size,theme,reading_mode,created_at,updated_at) VALUES($1,$2,$3,$4,$5,COALESCE($6,now()),COALESCE($7,now())) ON CONFLICT(reader_id) DO UPDATE SET font_size=EXCLUDED.font_size,theme=EXCLUDED.theme,reading_mode=EXCLUDED.reading_mode,updated_at=EXCLUDED.updated_at`, id, a, n1.Int64, s2, s3, nullableLegacyTime(t1), nullableLegacyTime(t2))
			}
		case "user_read_history":
			scanErr = rows.Scan(&id, &a, &b, &n1, &t1, &t2)
			if scanErr == nil {
				_, scanErr = target.ExecContext(ctx, `INSERT INTO reader_reading_history(id,reader_id,book_id,chapter_id,last_read_at,created_at,updated_at) VALUES($1,$2,$3,$4,COALESCE($5,now()),COALESCE($6,now()),COALESCE($6,now())) ON CONFLICT(reader_id,book_id) DO NOTHING`, id, a, b, n1.Int64, nullableLegacyTime(t2), nullableLegacyTime(t1))
			}
		case "reader_reading_history", "reader_read_history":
			scanErr = rows.Scan(&id, &a, &b, &c, &n1, &s1, &n1, &n1, &t1, &t2, &t3)
			if scanErr == nil {
				_, scanErr = target.ExecContext(ctx, `INSERT INTO reader_reading_history(id,reader_id,book_id,chapter_id,last_read_at,created_at,updated_at) VALUES($1,$2,$3,$4,COALESCE($5,now()),COALESCE($6,now()),COALESCE($7,now())) ON CONFLICT(reader_id,book_id) DO UPDATE SET chapter_id=EXCLUDED.chapter_id,last_read_at=EXCLUDED.last_read_at,updated_at=EXCLUDED.updated_at`, id, a, b, c, nullableLegacyTime(t1), nullableLegacyTime(t2), nullableLegacyTime(t3))
			}
		default:
			scanErr = rows.Scan(&id, &a, &b, &n1, &t1, &t2)
			if scanErr == nil {
				result.Errors = append(result.Errors, RecordError{SourceTable: table, SourceID: strconv.FormatInt(id, 10), Code: "UNSUPPORTED_LEGACY_SHAPE", Message: "legacy table requires explicit field mapping"})
			}
		}
		if scanErr != nil {
			return BatchResult{}, scanErr
		}
		result.Processed++
		result.NextCursor = fmt.Sprintf("%s:%d", table, id)
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	if result.Done {
		i := indexOf(readerCommerceTables, table)
		if i+1 < len(readerCommerceTables) {
			result.Done = false
			result.NextCursor = fmt.Sprintf("%s:0", readerCommerceTables[i+1])
		}
	}
	return result, nil
}

func nextCommerceCursor(table string) string {
	for i, value := range readerCommerceTables {
		if value == table && i+1 < len(readerCommerceTables) {
			return fmt.Sprintf("%s:0", readerCommerceTables[i+1])
		}
	}
	return ""
}
