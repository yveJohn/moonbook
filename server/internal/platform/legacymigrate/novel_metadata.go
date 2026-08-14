package legacymigrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type NovelCategoryDictionaryStage struct{}

func (NovelCategoryDictionaryStage) Name() string { return "novel-category-dictionary" }

func (NovelCategoryDictionaryStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	if exists, err := sourceTableExists(ctx, source, "sys_dict_data"); err != nil || !exists {
		if err != nil {
			return BatchResult{}, err
		}
		return BatchResult{}, errors.New("required legacy table sys_dict_data does not exist")
	}
	lastID, err := parseCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	rows, err := source.QueryContext(ctx, `
		SELECT dict_code, dict_value, dict_label, dict_type, COALESCE(dict_sort,0), COALESCE(status,'0')
		FROM sys_dict_data
		WHERE dict_type IN ('novel_book_category','novel_book_sub_category') AND dict_code > ?
		ORDER BY dict_code LIMIT ?`, lastID, limit)
	if err != nil {
		return BatchResult{}, err
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": "sys_dict_data"}}
	for rows.Next() {
		var id int64
		var code, name, dictType, status string
		var sort int
		if err := rows.Scan(&id, &code, &name, &dictType, &sort, &status); err != nil {
			return BatchResult{}, err
		}
		kind := "primary"
		if dictType == "novel_book_sub_category" {
			kind = "sub"
		}
		code, name = strings.TrimSpace(code), strings.TrimSpace(name)
		if code == "" || name == "" {
			result.Errors = append(result.Errors, RecordError{SourceTable: "sys_dict_data", SourceID: strconv.FormatInt(id, 10), Code: "INVALID_CATEGORY", Message: "category code or name is blank"})
		} else if _, err := target.ExecContext(ctx, `INSERT INTO novel_categories
			(id,code,name,kind,sort,enabled,source) VALUES ($1,$2,$3,$4,$5,$6,'legacy_dict')
			ON CONFLICT (id) DO UPDATE SET code=EXCLUDED.code,name=EXCLUDED.name,kind=EXCLUDED.kind,
			sort=EXCLUDED.sort,enabled=EXCLUDED.enabled,source='legacy_dict',updated_at=now(),deleted_at=NULL`,
			id, code, name, kind, sort, status == "0"); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(id, 10)
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	if err := syncIdentitySequence(ctx, target, "novel_categories"); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

type LegacyBookCategoryStage struct{}

func (LegacyBookCategoryStage) Name() string { return "legacy-book-category" }

func (LegacyBookCategoryStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	if exists, err := sourceTableExists(ctx, source, "book_category"); err != nil || !exists {
		if err != nil {
			return BatchResult{}, err
		}
		return BatchResult{Done: true, Metadata: map[string]any{"source": "book_category", "skipped": "table_not_found"}}, nil
	}
	lastID, err := parseCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	rows, err := source.QueryContext(ctx, `SELECT id, name, COALESCE(work_direction,''), COALESCE(sort,10)
		FROM book_category WHERE id > ? ORDER BY id LIMIT ?`, lastID, limit)
	if err != nil {
		return BatchResult{}, err
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": "book_category"}}
	for rows.Next() {
		var id int64
		var name, direction string
		var sort int
		if err := rows.Scan(&id, &name, &direction, &sort); err != nil {
			return BatchResult{}, err
		}
		name = strings.TrimSpace(name)
		if name == "" {
			result.Errors = append(result.Errors, RecordError{SourceTable: "book_category", SourceID: strconv.FormatInt(id, 10), Code: "INVALID_CATEGORY", Message: "category name is blank"})
		} else if _, err := target.ExecContext(ctx, `INSERT INTO novel_categories
			(id,code,name,kind,work_direction,sort,enabled,source) VALUES ($1,$2,$3,'primary',NULLIF($4,''),$5,true,'legacy_category')
			ON CONFLICT DO NOTHING`, id, strconv.FormatInt(id, 10), name, direction, sort); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(id, 10)
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	if err := syncIdentitySequence(ctx, target, "novel_categories"); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

type NovelBookAuthorStage struct{}

func (NovelBookAuthorStage) Name() string { return "novel-book-author" }

func (NovelBookAuthorStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	if exists, err := sourceTableExists(ctx, source, "novel_book"); err != nil || !exists {
		if err != nil {
			return BatchResult{}, err
		}
		return BatchResult{}, errors.New("required legacy table novel_book does not exist")
	}
	lastID, err := parseCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	rows, err := source.QueryContext(ctx, `SELECT id, author_id, author_name, COALESCE(work_direction,'')
		FROM novel_book WHERE id > ? ORDER BY id LIMIT ?`, lastID, limit)
	if err != nil {
		return BatchResult{}, err
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": "novel_book"}}
	for rows.Next() {
		var bookID int64
		var authorID sql.NullInt64
		var name, direction string
		if err := rows.Scan(&bookID, &authorID, &name, &direction); err != nil {
			return BatchResult{}, err
		}
		name = strings.TrimSpace(name)
		normalized := normalizeLegacyName(name)
		if name == "" || normalized == "" {
			result.Errors = append(result.Errors, RecordError{SourceTable: "novel_book", SourceID: strconv.FormatInt(bookID, 10), Code: "INVALID_AUTHOR", Message: "author name is blank"})
		} else if authorID.Valid && authorID.Int64 > 0 {
			_, err = target.ExecContext(ctx, `INSERT INTO novel_authors
				(id,pen_name,normalized_name,status,work_direction,source,legacy_book_author_id)
				VALUES ($1,$2,$3,'active',NULLIF($4,''),'legacy_book',$1)
				ON CONFLICT (id) DO UPDATE SET pen_name=EXCLUDED.pen_name,normalized_name=EXCLUDED.normalized_name,
				work_direction=EXCLUDED.work_direction,source='legacy_book',legacy_book_author_id=EXCLUDED.legacy_book_author_id,
				updated_at=now(),deleted_at=NULL`, authorID.Int64, name, normalized, direction)
			if err != nil {
				return BatchResult{}, err
			}
		} else {
			var existingID int64
			err = target.QueryRowContext(ctx, `SELECT id FROM novel_authors WHERE normalized_name=$1 AND source='legacy_book'
				AND legacy_author_id IS NULL AND legacy_book_author_id IS NULL AND deleted_at IS NULL ORDER BY id LIMIT 1`, normalized).Scan(&existingID)
			if errors.Is(err, sql.ErrNoRows) {
				_, err = target.ExecContext(ctx, `INSERT INTO novel_authors
					(pen_name,normalized_name,status,work_direction,source) VALUES ($1,$2,'active',NULLIF($3,''),'legacy_book')`, name, normalized, direction)
			}
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return BatchResult{}, err
			}
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(bookID, 10)
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	if err := syncIdentitySequence(ctx, target, "novel_authors"); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

type LegacyAuthorTableStage struct {
	Table string
}

func (stage LegacyAuthorTableStage) Name() string { return "legacy-" + stage.Table }

func (stage LegacyAuthorTableStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	if stage.Table != "author" && stage.Table != "book_author" {
		return BatchResult{}, fmt.Errorf("unsupported legacy author table %q", stage.Table)
	}
	if exists, err := sourceTableExists(ctx, source, stage.Table); err != nil || !exists {
		if err != nil {
			return BatchResult{}, err
		}
		return BatchResult{Done: true, Metadata: map[string]any{"source": stage.Table, "skipped": "table_not_found"}}, nil
	}
	lastID, err := parseCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	query := fmt.Sprintf("SELECT id, pen_name, COALESCE(work_direction,''), COALESCE(status,0) FROM %s WHERE id > ? ORDER BY id LIMIT ?", stage.Table)
	rows, err := source.QueryContext(ctx, query, lastID, limit)
	if err != nil {
		return BatchResult{}, err
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": stage.Table}}
	for rows.Next() {
		var id int64
		var name, direction string
		var oldStatus int
		if err := rows.Scan(&id, &name, &direction, &oldStatus); err != nil {
			return BatchResult{}, err
		}
		name = strings.TrimSpace(name)
		normalized := normalizeLegacyName(name)
		if name == "" || normalized == "" {
			result.Errors = append(result.Errors, RecordError{SourceTable: stage.Table, SourceID: strconv.FormatInt(id, 10), Code: "INVALID_AUTHOR", Message: "author name is blank"})
		} else if stage.Table == "book_author" {
			status := mapBookAuthorStatus(oldStatus)
			_, err = target.ExecContext(ctx, `INSERT INTO novel_authors
				(id,pen_name,normalized_name,status,work_direction,source,legacy_book_author_id)
				VALUES ($1,$2,$3,$4,NULLIF($5,''),'legacy_book_author',$1)
				ON CONFLICT (id) DO NOTHING`, id, name, normalized, status, direction)
		} else {
			status := "active"
			if oldStatus != 0 {
				status = "blocked"
			}
			_, err = target.ExecContext(ctx, `INSERT INTO novel_authors
				(id,pen_name,normalized_name,status,work_direction,source,legacy_author_id)
				VALUES ($1,$2,$3,$4,NULLIF($5,''),'legacy_author',$1)
				ON CONFLICT (id) DO UPDATE SET legacy_author_id=COALESCE(novel_authors.legacy_author_id,EXCLUDED.legacy_author_id),
				updated_at=now()`, id, name, normalized, status, direction)
		}
		if err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(id, 10)
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	if err := syncIdentitySequence(ctx, target, "novel_authors"); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

func sourceTableExists(ctx context.Context, source *sql.DB, table string) (bool, error) {
	var count int
	err := source.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.tables
		WHERE table_schema=DATABASE() AND table_name=? AND table_type='BASE TABLE'`, table).Scan(&count)
	return count > 0, err
}

func parseCursor(cursor string) (int64, error) {
	if cursor == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(cursor, 10, 64)
	if err != nil || value < 0 {
		return 0, fmt.Errorf("invalid migration cursor %q", cursor)
	}
	return value, nil
}

func normalizeLegacyName(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, strings.TrimSpace(value))
}

func mapBookAuthorStatus(status int) string {
	switch status {
	case 0:
		return "pending"
	case 1:
		return "active"
	default:
		return "blocked"
	}
}

func syncIdentitySequence(ctx context.Context, target *sql.Tx, table string) error {
	supported := map[string]bool{
		"novel_categories":           true,
		"novel_authors":              true,
		"novel_books":                true,
		"reader_invite_relations":    true,
		"reader_bookshelf_entries":   true,
		"reader_book_likes":          true,
		"reader_reading_history":     true,
		"reader_reading_preferences": true,
		"reader_feedback":            true,
	}
	if !supported[table] {
		return fmt.Errorf("unsupported identity table %q", table)
	}
	query := fmt.Sprintf(`SELECT setval(pg_get_serial_sequence('%s','id'), GREATEST((SELECT COALESCE(max(id),1) FROM %s),1), true)`, table, table)
	if _, err := target.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("sync %s identity sequence: %w", table, err)
	}
	return nil
}
