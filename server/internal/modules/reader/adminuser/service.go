package adminuser

import (
	"context"
	"errors"
)

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }
func (s *Service) List(ctx context.Context, k, st string, p, n int) ([]User, int64, error) {
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
func (s *Service) Get(ctx context.Context, id int64) (User, error) { return s.Repo.Get(ctx, id) }
func (s *Service) SetStatus(ctx context.Context, id int64, st string) (User, error) {
	return s.Repo.SetStatus(ctx, id, st)
}
func (s *Service) ResetPassword(ctx context.Context, id int64, password, confirm string) error {
	if len([]rune(password)) < 6 || len([]rune(password)) > 64 || password != confirm {
		return errors.New("invalid reader password")
	}
	return s.Repo.ResetPassword(ctx, id, password, confirm)
}
