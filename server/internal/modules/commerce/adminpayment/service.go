package adminpayment

import "context"

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service                      { return &Service{Repo: repo} }
func (s *Service) List(ctx context.Context) ([]Channel, error) { return s.Repo.List(ctx) }
func (s *Service) SetEnabled(ctx context.Context, id int64, enabled bool) (Channel, error) {
	return s.Repo.SetEnabled(ctx, id, enabled)
}
