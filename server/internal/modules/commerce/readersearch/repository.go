package readersearch

import (
	"context"
	"database/sql"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

type Repository interface {
	Upsert(context.Context, Projection) error
	Delete(context.Context, int64) error
	Page(context.Context, int64, int) (Page, error)
}

type SQLRepository struct {
	DB *sql.DB
}

func (repository SQLRepository) Upsert(ctx context.Context, projection Projection) error {
	_, err := transaction.Executor(ctx, repository.DB).ExecContext(ctx, `
INSERT INTO commerce_reader_search_projection (reader_id, username, nickname, status)
VALUES ($1, $2, $3, $4)
ON CONFLICT (reader_id) DO UPDATE
SET username = EXCLUDED.username,
    nickname = EXCLUDED.nickname,
    status = EXCLUDED.status,
    updated_at = now()`, projection.ReaderID, projection.Username, projection.Nickname, projection.Status)
	return err
}

func (repository SQLRepository) Delete(ctx context.Context, readerID int64) error {
	_, err := transaction.Executor(ctx, repository.DB).ExecContext(ctx, `
DELETE FROM commerce_reader_search_projection
WHERE reader_id = $1`, readerID)
	return err
}

func (repository SQLRepository) Page(ctx context.Context, afterReaderID int64, limit int) (Page, error) {
	rows, err := transaction.Executor(ctx, repository.DB).QueryContext(ctx, `
SELECT reader_id, username, nickname, status, created_at, updated_at
FROM commerce_reader_search_projection
WHERE reader_id > $1
ORDER BY reader_id
LIMIT $2`, afterReaderID, limit+1)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()

	items := make([]Projection, 0, limit+1)
	for rows.Next() {
		var projection Projection
		if err := rows.Scan(
			&projection.ReaderID,
			&projection.Username,
			&projection.Nickname,
			&projection.Status,
			&projection.CreatedAt,
			&projection.UpdatedAt,
		); err != nil {
			return Page{}, err
		}
		items = append(items, projection)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}

	done := len(items) <= limit
	if !done {
		items = items[:limit]
	}
	nextReaderID := afterReaderID
	if len(items) > 0 {
		nextReaderID = items[len(items)-1].ReaderID
	}
	return Page{
		Items:         items,
		AfterReaderID: afterReaderID,
		Limit:         limit,
		NextReaderID:  nextReaderID,
		Done:          done,
	}, nil
}
