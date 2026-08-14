package adminproduct

import "context"

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }
func (s *Service) List(ctx context.Context, keyword, productType, status string, page, size int) ([]Product, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return s.Repo.List(ctx, keyword, productType, status, page, size)
}
func (s *Service) Create(ctx context.Context, in Input) (Product, error) {
	return s.Repo.Create(ctx, in)
}
func (s *Service) Update(ctx context.Context, id int64, in Input) (Product, error) {
	return s.Repo.Update(ctx, id, in)
}
func (s *Service) Delete(ctx context.Context, id int64) error { return s.Repo.Delete(ctx, id) }
