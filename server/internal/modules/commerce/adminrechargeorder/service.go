package adminrechargeorder

import (
	"context"
	"errors"
	"net/http"
	"time"

	readercontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

type Service struct {
	Repo               Repository
	Tx                 Transactor
	Reader             readercontract.AccountLocker
	CallbackStaleAfter time.Duration
}

func NewService(repo Repository, tx Transactor, reader readercontract.AccountLocker, staleAfter ...time.Duration) *Service {
	threshold := 5 * time.Minute
	if len(staleAfter) > 0 && staleAfter[0] > 0 {
		threshold = staleAfter[0]
	}
	return &Service{Repo: repo, Tx: tx, Reader: reader, CallbackStaleAfter: threshold}
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
func (s *Service) ListCallbacks(ctx context.Context, filter CallbackFilter) ([]CallbackLog, int64, error) {
	r, ok := s.Repo.(CallbackRepository)
	if !ok {
		return nil, 0, errors.New("callback log repository unavailable")
	}
	if err := validateCallbackFilter(filter); err != nil {
		return nil, 0, err
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	rows, total, err := r.ListCallbacks(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	for index := range rows {
		s.markInterrupted(&rows[index], time.Now().UTC())
	}
	return rows, total, nil
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
	item, err := r.GetCallback(ctx, id)
	if err == nil {
		s.markInterrupted(&item, time.Now().UTC())
	}
	return item, err
}

func (s *Service) markInterrupted(item *CallbackLog, now time.Time) {
	item.Interrupted = item.SourceType == "runtime" && item.ProcessingResult == "received" && item.RequestTime.Before(now.Add(-s.CallbackStaleAfter))
}

func validateCallbackFilter(filter CallbackFilter) error {
	if filter.SignatureStatus != "" && filter.SignatureStatus != "valid" && filter.SignatureStatus != "invalid" && filter.SignatureStatus != "not_checked" {
		return invalidCallbackFilter("签名状态无效")
	}
	if filter.ProcessingResult != "" && !validLowerCode(filter.ProcessingResult) {
		return invalidCallbackFilter("处理结果无效")
	}
	if filter.FailureCode != "" && !validUpperCode(filter.FailureCode) {
		return invalidCallbackFilter("失败码无效")
	}
	if filter.ResponseStatus != nil && (*filter.ResponseStatus < 0 || *filter.ResponseStatus > 599) {
		return invalidCallbackFilter("响应状态无效")
	}
	if filter.StartTime != nil && filter.EndTime != nil {
		if filter.EndTime.Before(*filter.StartTime) {
			return invalidCallbackFilter("结束时间不能早于开始时间")
		}
		if filter.EndTime.Sub(*filter.StartTime) > 31*24*time.Hour {
			return invalidCallbackFilter("查询时间跨度不能超过31天")
		}
	}
	return nil
}

func invalidCallbackFilter(message string) error {
	return apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, message)
}

func validLowerCode(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if character < 'a' || character > 'z' {
			if character != '_' && (character < '0' || character > '9') {
				return false
			}
		}
	}
	return true
}

func validUpperCode(value string) bool {
	if len(value) == 0 || len(value) > 64 || value[0] < 'A' || value[0] > 'Z' {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			if character != '_' && (character < '0' || character > '9') {
				return false
			}
		}
	}
	return true
}
