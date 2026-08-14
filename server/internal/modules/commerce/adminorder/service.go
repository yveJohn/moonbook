package adminorder

import "context"

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
