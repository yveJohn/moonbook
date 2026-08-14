package admininvite

import "context"

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }
func (s *Service) List(ctx context.Context, k string, p, n int) ([]InviteCode, int64, error) {
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
func (s *Service) Create(ctx context.Context, in CreateInput) (InviteCode, error) {
	return s.Repo.Create(ctx, in)
}
func (s *Service) SetStatus(ctx context.Context, id int64, st string) (InviteCode, error) {
	return s.Repo.SetStatus(ctx, id, st)
}
func (s *Service) Delete(ctx context.Context, id int64) error { return s.Repo.Delete(ctx, id) }
