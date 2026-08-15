package provider

import (
	"context"
	"database/sql"
	"errors"

	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

type Purchase struct{ db *sql.DB }

var (
	_ novelcontract.ProductTargetReader    = (*Purchase)(nil)
	_ novelcontract.PurchaseSnapshotReader = (*Purchase)(nil)
)

func NewPurchase(db *sql.DB) *Purchase { return &Purchase{db: db} }

func (provider *Purchase) ProductTarget(ctx context.Context, targetType string, targetID int64) (novelcontract.TargetSnapshot, error) {
	if provider == nil || provider.db == nil || targetID <= 0 {
		return novelcontract.TargetSnapshot{}, targetNotFound(targetType)
	}
	executor := transaction.Executor(ctx, provider.db)
	switch targetType {
	case "book":
		var snapshot novelcontract.TargetSnapshot
		var publishStatus string
		var deletedAt sql.NullTime
		err := executor.QueryRowContext(ctx, `SELECT id,id,book_name,publish_status,deleted_at FROM novel_books WHERE id=$1`, targetID).
			Scan(&snapshot.TargetID, &snapshot.BookID, &snapshot.Name, &publishStatus, &deletedAt)
		if err != nil {
			return novelcontract.TargetSnapshot{}, classifyTargetError(targetType, err)
		}
		snapshot.Type = targetType
		snapshot.Enabled = publishStatus == "published" && !deletedAt.Valid
		return snapshot, nil
	case "chapter":
		var snapshot novelcontract.TargetSnapshot
		var chapterStatus, publishStatus string
		var chapterDeletedAt, bookDeletedAt sql.NullTime
		err := executor.QueryRowContext(ctx, `SELECT c.id,c.book_id,c.chapter_name,c.chapter_status,c.deleted_at,b.publish_status,b.deleted_at FROM novel_chapters c JOIN novel_books b ON b.id=c.book_id WHERE c.id=$1`, targetID).
			Scan(&snapshot.TargetID, &snapshot.BookID, &snapshot.Name, &chapterStatus, &chapterDeletedAt, &publishStatus, &bookDeletedAt)
		if err != nil {
			return novelcontract.TargetSnapshot{}, classifyTargetError(targetType, err)
		}
		snapshot.Type = targetType
		snapshot.Enabled = chapterStatus == "enabled" && !chapterDeletedAt.Valid && publishStatus == "published" && !bookDeletedAt.Valid
		return snapshot, nil
	default:
		return novelcontract.TargetSnapshot{}, novelcontract.ErrTargetUnavailable
	}
}

func (provider *Purchase) LockPurchaseSnapshot(ctx context.Context, targetType string, targetID int64) (novelcontract.PurchaseSnapshot, error) {
	if provider == nil || provider.db == nil || targetID <= 0 {
		return novelcontract.PurchaseSnapshot{}, targetNotFound(targetType)
	}
	executor := transaction.Executor(ctx, provider.db)
	if executor == provider.db {
		return novelcontract.PurchaseSnapshot{}, novelcontract.Wrap(novelcontract.ErrUnavailable, transaction.ErrNoTransaction)
	}
	switch targetType {
	case "book":
		var snapshot novelcontract.PurchaseSnapshot
		var publishStatus string
		var deletedAt sql.NullTime
		err := executor.QueryRowContext(ctx, `SELECT id,id,book_name,word_count,publish_status,deleted_at FROM novel_books WHERE id=$1 FOR UPDATE`, targetID).
			Scan(&snapshot.TargetID, &snapshot.BookID, &snapshot.Name, &snapshot.WordCount, &publishStatus, &deletedAt)
		if err != nil {
			return novelcontract.PurchaseSnapshot{}, classifyPurchaseError(targetType, err)
		}
		snapshot.Type = targetType
		snapshot.Enabled = publishStatus == "published" && !deletedAt.Valid
		return snapshot, nil
	case "chapter":
		var snapshot novelcontract.PurchaseSnapshot
		var chapterStatus, publishStatus string
		var chapterDeletedAt, bookDeletedAt sql.NullTime
		err := executor.QueryRowContext(ctx, `SELECT c.id,c.book_id,c.chapter_name,c.word_count,c.chapter_status,c.deleted_at,b.publish_status,b.deleted_at FROM novel_chapters c JOIN novel_books b ON b.id=c.book_id WHERE c.id=$1 FOR UPDATE OF b,c`, targetID).
			Scan(&snapshot.TargetID, &snapshot.BookID, &snapshot.Name, &snapshot.WordCount, &chapterStatus, &chapterDeletedAt, &publishStatus, &bookDeletedAt)
		if err != nil {
			return novelcontract.PurchaseSnapshot{}, classifyPurchaseError(targetType, err)
		}
		snapshot.Type = targetType
		snapshot.Enabled = chapterStatus == "enabled" && !chapterDeletedAt.Valid && publishStatus == "published" && !bookDeletedAt.Valid
		return snapshot, nil
	default:
		return novelcontract.PurchaseSnapshot{}, novelcontract.ErrTargetUnavailable
	}
}

func targetNotFound(targetType string) error {
	if targetType == "book" {
		return novelcontract.ErrBookNotFound
	}
	if targetType == "chapter" {
		return novelcontract.ErrChapterNotFound
	}
	return novelcontract.ErrTargetUnavailable
}

func classifyTargetError(targetType string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return targetNotFound(targetType)
	}
	return novelcontract.Wrap(novelcontract.ErrUnavailable, err)
}

func classifyPurchaseError(targetType string, err error) error {
	return classifyTargetError(targetType, err)
}
