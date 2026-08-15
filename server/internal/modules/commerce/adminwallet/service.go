package adminwallet

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	readercontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

type Service struct {
	Repo   Repository
	Tx     Transactor
	Reader readercontract.AccountLocker
}

func NewService(repo Repository, tx Transactor, reader readercontract.AccountLocker) *Service {
	return &Service{Repo: repo, Tx: tx, Reader: reader}
}
func (s *Service) ListWallets(ctx context.Context, k string, p, n int) ([]Wallet, int64, error) {
	if p < 1 {
		p = 1
	}
	if n < 1 {
		n = 20
	}
	if n > 100 {
		n = 100
	}
	return s.Repo.ListWallets(ctx, k, p, n)
}
func (s *Service) ListLedgers(ctx context.Context, id int64, c string, p, n int) ([]Ledger, int64, error) {
	if id <= 0 {
		return nil, 0, errors.New("invalid reader id")
	}
	if p < 1 {
		p = 1
	}
	if n < 1 {
		n = 20
	}
	if n > 100 {
		n = 100
	}
	return s.Repo.ListLedgers(ctx, id, c, p, n)
}
func (s *Service) Adjust(ctx context.Context, in AdjustmentInput) (Adjustment, error) {
	readerID, err := strconv.ParseInt(strings.TrimSpace(in.ReaderID), 10, 64)
	if err != nil || readerID <= 0 {
		return Adjustment{}, ErrInvalidAdjustment
	}
	if s.Repo == nil || s.Tx == nil || s.Reader == nil {
		return Adjustment{}, apperror.New(apperror.CodeUnavailable, http.StatusServiceUnavailable, "读者服务暂不可用")
	}
	var adjustment Adjustment
	err = s.Tx.Within(ctx, func(txCtx context.Context) error {
		if _, err := s.Reader.LockAccount(txCtx, readerID); err != nil {
			return err
		}
		var err error
		adjustment, err = s.Repo.Adjust(txCtx, in)
		return err
	})
	if err != nil {
		return Adjustment{}, readerError(err)
	}
	return adjustment, nil
}

func readerError(err error) error {
	if errors.Is(err, readercontract.ErrAccountNotFound) {
		return apperror.Wrap(err, apperror.CodeNotFound, http.StatusNotFound, "读者不存在")
	}
	if errors.Is(err, readercontract.ErrAccountDisabled) {
		return apperror.Wrap(err, apperror.CodeConflict, http.StatusConflict, "读者账号已禁用")
	}
	if errors.Is(err, readercontract.ErrUnavailable) {
		return apperror.Wrap(err, apperror.CodeUnavailable, http.StatusServiceUnavailable, "读者服务暂不可用")
	}
	return err
}
