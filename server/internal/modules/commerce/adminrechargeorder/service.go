package adminrechargeorder

import (
	"context"
	"errors"
)

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }
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
	return s.Repo.ManualPay(ctx, id, in)
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
func (s *Service) GetCallback(ctx context.Context, id int64) (CallbackLog, error) {
	r, ok := s.Repo.(CallbackRepository)
	if !ok {
		return CallbackLog{}, errors.New("callback log repository unavailable")
	}
	return r.GetCallback(ctx, id)
}
