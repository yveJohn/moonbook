package candidate

import (
	"context"
	"errors"
	"strings"
)

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }
func (s *Service) List(ctx context.Context, keyword, sourceID, boardID, status string, page, size int) ([]Candidate, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return s.Repo.List(ctx, keyword, sourceID, boardID, status, page, size)
}
func (s *Service) Get(ctx context.Context, id int64) (Candidate, error) {
	if id <= 0 {
		return Candidate{}, errors.New("invalid candidate id")
	}
	return s.Repo.Get(ctx, id)
}
func (s *Service) Skip(ctx context.Context, ids []int64, requestID string) error {
	return s.transition(ctx, ids, "skipped", requestID)
}
func (s *Service) Restore(ctx context.Context, ids []int64, requestID string) error {
	return s.transition(ctx, ids, "pending", requestID)
}
func (s *Service) transition(ctx context.Context, ids []int64, status, requestID string) error {
	if len(ids) == 0 || len(ids) > 100 {
		return errors.New("invalid candidate ids")
	}
	return s.Repo.Transition(ctx, ids, status, requestID)
}
func (s *Service) Delete(ctx context.Context, ids []int64) error {
	if len(ids) == 0 || len(ids) > 100 {
		return errors.New("invalid candidate ids")
	}
	return s.Repo.Delete(ctx, ids)
}

func (s *Service) Discover(ctx context.Context, boardID string) (DiscoverResult, error) {
	boardID = strings.TrimSpace(boardID)
	if boardID == "" {
		return DiscoverResult{}, errors.New("invalid board id")
	}
	target, err := s.Repo.GetBoardTarget(ctx, boardID)
	if err != nil {
		return DiscoverResult{}, err
	}
	if !target.Enabled {
		return DiscoverResult{}, errors.New("board or source is disabled")
	}
	items, err := DiscoverBoard(ctx, target, nil)
	if err != nil {
		return DiscoverResult{}, err
	}
	return s.Repo.UpsertDiscovered(ctx, target, items)
}
