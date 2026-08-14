package legacymigrate

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type ReaderActivityStage struct{}

func (ReaderActivityStage) Name() string { return "reader-activity" }

func (ReaderActivityStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	exists, err := sourceTableExists(ctx, source, "reader_daily_activity")
	if err != nil {
		return BatchResult{}, err
	}
	if !exists {
		return BatchResult{Done: true, Metadata: map[string]any{"source": "reader_daily_activity", "skipped": "table_not_found"}}, nil
	}
	readerID, day, err := activityCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	rows, err := source.QueryContext(ctx, `
		SELECT reader_id,DATE_FORMAT(activity_date,'%Y-%m-%d'),DATE_FORMAT(first_active_time,'%Y-%m-%d %H:%i:%s'),DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s')
		FROM reader_daily_activity
		WHERE reader_id>? OR (reader_id=? AND activity_date>?)
		ORDER BY reader_id,activity_date LIMIT ?`, readerID, readerID, day, limit)
	if err != nil {
		return BatchResult{}, err
	}
	defer rows.Close()
	zone, err := time.LoadLocation("Asia/Kuala_Lumpur")
	if err != nil {
		return BatchResult{}, err
	}
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": "reader_daily_activity"}}
	for rows.Next() {
		var id int64
		var activityDate, firstRaw, createdRaw string
		if err := rows.Scan(&id, &activityDate, &firstRaw, &createdRaw); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = fmt.Sprintf("%d|%s", id, activityDate)
		first, firstErr := time.ParseInLocation("2006-01-02 15:04:05", firstRaw, zone)
		created, createdErr := time.ParseInLocation("2006-01-02 15:04:05", createdRaw, zone)
		if firstErr != nil || createdErr != nil || first.Format(time.DateOnly) != activityDate {
			result.Errors = append(result.Errors, RecordError{SourceTable: "reader_daily_activity", SourceID: result.NextCursor, Code: "INVALID_READER_ACTIVITY", Message: "legacy reader activity date or timestamp is invalid"})
			continue
		}
		var accountExists bool
		if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reader_accounts WHERE id=$1)`, id).Scan(&accountExists); err != nil {
			return BatchResult{}, err
		}
		if !accountExists {
			result.Errors = append(result.Errors, RecordError{SourceTable: "reader_daily_activity", SourceID: result.NextCursor, Code: "MISSING_READER_ACTIVITY_ACCOUNT", Message: "legacy activity references an unavailable reader account"})
			continue
		}
		if _, err := target.ExecContext(ctx, `
			INSERT INTO reader_daily_activity(activity_date,reader_id,first_active_at,source_type,created_at)
			VALUES($1,$2,$3,'legacy',$4) ON CONFLICT(activity_date,reader_id) DO NOTHING`, activityDate, id, first, created); err != nil {
			return BatchResult{}, err
		}
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

func activityCursor(raw string) (int64, string, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, "0001-01-01", nil
	}
	parts := strings.Split(raw, "|")
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid reader activity cursor %q", raw)
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id < 0 {
		return 0, "", fmt.Errorf("invalid reader activity cursor %q", raw)
	}
	if _, err = time.Parse(time.DateOnly, parts[1]); err != nil {
		return 0, "", fmt.Errorf("invalid reader activity cursor %q", raw)
	}
	return id, parts[1], nil
}
