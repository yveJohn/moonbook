package adminrechargeorder

import (
	"context"
	"errors"
	"net/http"

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
func (s *Service) List(ctx context.Context, k, st string, p, n int) ([]Order, int64, error) {
	if p < 1 {
		p = 1
	}
	if n < 1 {
		n = 20
	}
	if n > 100 {
		n = 100
	}
	return s.Repo.List(ctx, k, st, p, n)
}
func (s *Service) Get(ctx context.Context, id int64) (Order, error) { return s.Repo.Get(ctx, id) }
func (s *Service) ManualPay(ctx context.Context, id int64, in ManualPayInput) (Order, error) {
	if err := validateManualPay(id, &in); err != nil {
		return Order{}, err
	}
	if s.Repo == nil || s.Tx == nil || s.Reader == nil {
		return Order{}, apperror.New(apperror.CodeUnavailable, http.StatusServiceUnavailable, "读者服务暂不可用")
	}
	readerID, err := s.Repo.ReaderID(ctx, id)
	if err != nil {
		return Order{}, err
	}
	var order Order
	err = s.Tx.Within(ctx, func(txCtx context.Context) error {
		if _, err := s.Reader.LockAccount(txCtx, readerID); err != nil {
			return err
		}
		var err error
		order, err = s.Repo.ManualPay(txCtx, id, readerID, in)
		return err
	})
	if err != nil {
		return Order{}, readerError(err)
	}
	return order, nil
}
func (s *Service) Sync(ctx context.Context, id int64) (Order, error) { return s.Repo.Sync(ctx, id) }
func (s *Service) ListCallbacks(ctx context.Context, keyword, result string, page, size int) ([]CallbackLog, int64, error) {
	r, ok := s.Repo.(CallbackRepository)
	if !ok {
		return nil, 0, errors.New("callback log repository unavailable")
	}
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return r.ListCallbacks(ctx, keyword, result, page, size)
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
func (s *Service) GetCallback(ctx context.Context, id int64) (CallbackLog, error) {
	r, ok := s.Repo.(CallbackRepository)
	if !ok {
		return CallbackLog{}, errors.New("callback log repository unavailable")
	}
	return r.GetCallback(ctx, id)
}
