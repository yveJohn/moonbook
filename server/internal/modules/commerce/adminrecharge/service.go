package adminrecharge

import "context"

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }
func (s *Service) List(ctx context.Context, k string, p, n int) ([]Product, int64, error) {
	if p < 1 {
		p = 1
	}
	if n < 1 {
		n = 20
	}
	if n > 100 {
		n = 100
	}
	return s.Repo.List(ctx, k, p, n)
}
func (s *Service) Create(ctx context.Context, in Input) (Product, error) {
	return s.Repo.Create(ctx, in)
}
func (s *Service) Update(ctx context.Context, id int64, in Input) (Product, error) {
	return s.Repo.Update(ctx, id, in)
}
func (s *Service) Delete(ctx context.Context, id int64) error { return s.Repo.Delete(ctx, id) }
