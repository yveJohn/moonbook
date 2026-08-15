package purchase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
	readercontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/contract"
)

type Service struct {
	Repo   Repository
	Tx     Transactor
	Reader readercontract.AccountLocker
	Novel  novelcontract.PurchaseSnapshotReader
}

func NewService(repo Repository, tx Transactor, reader readercontract.AccountLocker, novel novelcontract.PurchaseSnapshotReader) *Service {
	return &Service{Repo: repo, Tx: tx, Reader: reader, Novel: novel}
}

func (s *Service) BuyMembership(ctx context.Context, readerID int64, productID, requestID string) (Order, error) {
	if err := validateRequest(readerID, requestID); err != nil {
		return Order{}, err
	}
	pid, err := parseID(productID)
	if err != nil {
		return Order{}, err
	}
	if !s.ready(false) {
		return Order{}, ErrUnavailable
	}
	key := "membership:" + requestID
	var order Order
	err = s.Tx.Within(ctx, func(txCtx context.Context) error {
		if err := s.Repo.LockIdempotency(txCtx, readerID); err != nil {
			return err
		}
		existing, err := s.Repo.FindOrder(txCtx, readerID, key)
		if err == nil {
			order = existing
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err = s.Reader.LockAccount(txCtx, readerID); err != nil {
			return err
		}
		order, err = s.Repo.BuyMembership(txCtx, readerID, pid, key)
		return err
	})
	if err != nil {
		return Order{}, purchaseDependencyError(err)
	}
	return order, nil
}

func (s *Service) BuyChapter(ctx context.Context, readerID int64, chapterID, expectedPrice, requestID string) (ChapterResult, error) {
	if err := validateRequest(readerID, requestID); err != nil {
		return ChapterResult{}, err
	}
	cid, err := parseID(chapterID)
	if err != nil {
		return ChapterResult{}, err
	}
	expected, err := strconv.ParseInt(expectedPrice, 10, 64)
	if err != nil || expected < 0 {
		return ChapterResult{}, ErrInvalidRequest
	}
	if !s.ready(true) {
		return ChapterResult{}, ErrUnavailable
	}
	key := "chapter:" + requestID
	var result ChapterResult
	err = s.Tx.Within(ctx, func(txCtx context.Context) error {
		if err := s.Repo.LockIdempotency(txCtx, readerID); err != nil {
			return err
		}
		existing, err := s.Repo.FindOrder(txCtx, readerID, key)
		if err == nil {
			result = ChapterResult{PurchaseStatus: "paid", Order: &existing}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err = s.Reader.LockAccount(txCtx, readerID); err != nil {
			return err
		}
		snapshot, err := s.Novel.LockPurchaseSnapshot(txCtx, "chapter", cid)
		if err != nil {
			return err
		}
		if !validSnapshot(snapshot, "chapter", cid) {
			return ErrProductUnavailable
		}
		result, err = s.Repo.BuyChapter(txCtx, readerID, snapshot, expected, key)
		return err
	})
	if err != nil {
		return ChapterResult{}, purchaseDependencyError(err)
	}
	return result, nil
}

func (s *Service) BuyBook(ctx context.Context, readerID int64, bookID, expectedPrice string) (Order, error) {
	if err := validateRequest(readerID, expectedPrice); err != nil {
		return Order{}, err
	}
	bid, err := parseID(bookID)
	if err != nil {
		return Order{}, err
	}
	expected, err := strconv.ParseInt(expectedPrice, 10, 64)
	if err != nil || expected < 0 {
		return Order{}, ErrInvalidRequest
	}
	if !s.ready(true) {
		return Order{}, ErrUnavailable
	}
	key := fmt.Sprintf("book:%d:%s", bid, expectedPrice)
	var order Order
	err = s.Tx.Within(ctx, func(txCtx context.Context) error {
		if err := s.Repo.LockIdempotency(txCtx, readerID); err != nil {
			return err
		}
		existing, err := s.Repo.FindOrder(txCtx, readerID, key)
		if err == nil {
			order = existing
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err = s.Reader.LockAccount(txCtx, readerID); err != nil {
			return err
		}
		snapshot, err := s.Novel.LockPurchaseSnapshot(txCtx, "book", bid)
		if err != nil {
			return err
		}
		if !validSnapshot(snapshot, "book", bid) {
			return ErrProductUnavailable
		}
		order, err = s.Repo.BuyBook(txCtx, readerID, snapshot, expected, key)
		return err
	})
	if err != nil {
		return Order{}, purchaseDependencyError(err)
	}
	return order, nil
}

func (s *Service) ready(requireNovel bool) bool {
	return s != nil && s.Repo != nil && s.Tx != nil && s.Reader != nil && (!requireNovel || s.Novel != nil)
}

func validSnapshot(snapshot novelcontract.PurchaseSnapshot, targetType string, targetID int64) bool {
	return snapshot.Enabled && snapshot.Type == targetType && snapshot.TargetID == targetID && snapshot.BookID > 0
}

func purchaseDependencyError(err error) error {
	switch {
	case errors.Is(err, readercontract.ErrAccountNotFound), errors.Is(err, readercontract.ErrAccountDisabled):
		return ErrProductUnavailable
	case errors.Is(err, novelcontract.ErrBookNotFound), errors.Is(err, novelcontract.ErrChapterNotFound), errors.Is(err, novelcontract.ErrNotPublished), errors.Is(err, novelcontract.ErrTargetUnavailable):
		return ErrProductUnavailable
	default:
		return err
	}
}
