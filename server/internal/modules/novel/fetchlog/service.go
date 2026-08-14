package fetchlog

import (
	"context"
	"errors"
)

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }
func (s *Service) List(ctx context.Context, keyword, status, stage, taskID string, page, size int) ([]Log, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return s.Repo.List(ctx, keyword, status, stage, taskID, page, size)
}
func (s *Service) Get(ctx context.Context, id int64) (Log, error) {
	if id <= 0 {
		return Log{}, errors.New("invalid fetch log id")
	}
	return s.Repo.Get(ctx, id)
}
