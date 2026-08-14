package adminorder

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }
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
	return s.Repo.CreateMockRecharge(ctx, in, operatorID)
}
func (s *Service) ConfirmMockRecharge(ctx context.Context, id, operatorID int64) (Order, error) {
	if id <= 0 || operatorID <= 0 {
		return Order{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "模拟充值确认参数无效")
	}
	return s.Repo.ConfirmMockRecharge(ctx, id, operatorID)
}
