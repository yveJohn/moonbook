package adminwallet

import (
	"context"
	"errors"
)

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }
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
