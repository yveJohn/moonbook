package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

type Likes struct{ db *sql.DB }

var (
	_ novelcontract.LikeBookLocker    = (*Likes)(nil)
	_ novelcontract.LikeSummaryWriter = (*Likes)(nil)
)

func NewLikes(db *sql.DB) *Likes { return &Likes{db: db} }

func (provider *Likes) LockPublishedBook(ctx context.Context, bookID int64) error {
	if provider == nil || provider.db == nil || bookID <= 0 {
		return novelcontract.ErrBookNotFound
	}
	executor := transaction.Executor(ctx, provider.db)
	if executor == provider.db {
		return novelcontract.Wrap(novelcontract.ErrUnavailable, transaction.ErrNoTransaction)
	}
	lockKey := fmt.Sprintf("novel-like:%d", bookID)
	if _, err := executor.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lockKey); err != nil {
		return novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	var status string
	var deletedAt sql.NullTime
	if err := executor.QueryRowContext(ctx, `SELECT publish_status,deleted_at FROM novel_books WHERE id=$1 FOR UPDATE`, bookID).Scan(&status, &deletedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return novelcontract.ErrBookNotFound
		}
		return novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	if status != "published" || deletedAt.Valid {
		return novelcontract.ErrNotPublished
	}
	return nil
}

func (provider *Likes) SetLikeCount(ctx context.Context, bookID, count int64) error {
	if provider == nil || provider.db == nil || bookID <= 0 || count < 0 {
		return novelcontract.ErrUnknown
	}
	executor := transaction.Executor(ctx, provider.db)
	if executor == provider.db {
		return novelcontract.Wrap(novelcontract.ErrUnavailable, transaction.ErrNoTransaction)
	}
	result, err := executor.ExecContext(ctx, `UPDATE novel_books SET like_count=$2,updated_at=now() WHERE id=$1 AND publish_status='published' AND deleted_at IS NULL`, bookID, count)
	if err != nil {
		return novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	if rows != 1 {
		return novelcontract.ErrNotPublished
	}
	return nil
}
