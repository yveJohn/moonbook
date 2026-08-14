package checkin

import "context"

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }
func (s *Service) Status(ctx context.Context, id int64) (Status, error) {
	return s.Repo.Status(ctx, id)
}
func (s *Service) Checkin(ctx context.Context, id int64) (Status, error) {
	return s.Repo.Checkin(ctx, id)
}
