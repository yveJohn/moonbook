package payment

import "context"

type Service struct {
	Repo        Repository
	PID, Secret string
}

func NewService(repo Repository, pid, secret string) *Service {
	return &Service{Repo: repo, PID: pid, Secret: secret}
}
func (s *Service) Process(ctx context.Context, c Callback) error { return s.Repo.Process(ctx, c) }
