package adminorder

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
func (s *Service) List(ctx context.Context, keyword, orderType, status string, page, size int) ([]Order, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return s.Repo.List(ctx, keyword, orderType, status, page, size)
}
func (s *Service) Get(ctx context.Context, id int64) (Order, error) { return s.Repo.Get(ctx, id) }
func (s *Service) CreateMockRecharge(ctx context.Context, in MockRechargeInput, operatorID int64) (Order, error) {
	in.ReaderID = strings.TrimSpace(in.ReaderID)
	in.RechargeCoinAmount = strings.TrimSpace(in.RechargeCoinAmount)
	in.RequestID = strings.TrimSpace(in.RequestID)
	in.Remark = strings.TrimSpace(in.Remark)
	readerID, readerErr := strconv.ParseInt(in.ReaderID, 10, 64)
	amount, amountErr := strconv.ParseInt(in.RechargeCoinAmount, 10, 64)
	if readerErr != nil || readerID <= 0 || amountErr != nil || amount <= 0 || in.RequestID == "" || len(in.RequestID) > 100 || len([]rune(in.Remark)) > 255 || operatorID <= 0 {
		return Order{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "模拟充值参数无效")
	}
	if s.Repo == nil || s.Tx == nil || s.Reader == nil {
		return Order{}, apperror.New(apperror.CodeUnavailable, http.StatusServiceUnavailable, "读者服务暂不可用")
	}
	var order Order
	err := s.Tx.Within(ctx, func(txCtx context.Context) error {
		if _, err := s.Reader.LockAccount(txCtx, readerID); err != nil {
			return err
		}
		var err error
		order, err = s.Repo.CreateMockRecharge(txCtx, in, operatorID)
		return err
	})
	if err != nil {
		return Order{}, readerError(err)
	}
	return order, nil
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
func (s *Service) ConfirmMockRecharge(ctx context.Context, id, operatorID int64) (Order, error) {
	if id <= 0 || operatorID <= 0 {
		return Order{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "模拟充值确认参数无效")
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
		order, err = s.Repo.ConfirmMockRecharge(txCtx, id, operatorID)
		return err
	})
	if err != nil {
		return Order{}, readerError(err)
	}
	return order, nil
}
