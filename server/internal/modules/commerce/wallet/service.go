package wallet

import (
	"context"
	"errors"
)

var ErrRepositoryUnavailable = errors.New("wallet repository unavailable")

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }

func (s *Service) Get(ctx context.Context, readerID int64) (Wallet, error) {
	if s == nil || s.Repo == nil {
		return Wallet{}, ErrRepositoryUnavailable
	}
	return s.Repo.Get(ctx, readerID)
}

func (s *Service) ListByReaderIDs(ctx context.Context, readerIDs []int64) ([]Wallet, error) {
	if s == nil || s.Repo == nil {
		return nil, ErrRepositoryUnavailable
	}
	return s.Repo.ListByReaderIDs(ctx, readerIDs)
}

func (s *Service) List(ctx context.Context, readerID int64, coinType string, page, size int) ([]Ledger, int64, error) {
	if s == nil || s.Repo == nil {
		return nil, 0, ErrRepositoryUnavailable
	}
	if coinType != "recharge" && coinType != "bonus" {
		return nil, 0, errors.New("invalid coin type")
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
	return s.Repo.List(ctx, readerID, coinType, page, size)
}
