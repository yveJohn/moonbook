package importtask

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }
func (s *Service) List(ctx context.Context, keyword, status, sourceID, boardID string, page, size int) ([]Task, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return s.Repo.List(ctx, keyword, status, sourceID, boardID, page, size)
}
func (s *Service) Get(ctx context.Context, id int64) (Task, error) {
	if id <= 0 {
		return Task{}, errors.New("invalid import task id")
	}
	return s.Repo.Get(ctx, id)
}
func normalize(in CreateInput) CreateInput {
	in.CandidateID = strings.TrimSpace(in.CandidateID)
	in.DisplayTitle = strings.TrimSpace(in.DisplayTitle)
	in.TargetBookID = strings.TrimSpace(in.TargetBookID)
	in.ImportMode = strings.TrimSpace(in.ImportMode)
	in.MergeStrategy = strings.TrimSpace(in.MergeStrategy)
	in.OperatorName = strings.TrimSpace(in.OperatorName)
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	return in
}
func valid(in CreateInput) error {
	if n, e := strconv.ParseInt(in.CandidateID, 10, 64); e != nil || n <= 0 {
		return errors.New("invalid candidate id")
	}
	if in.ImportMode != "create" && in.ImportMode != "incremental" {
		return errors.New("invalid import mode")
	}
	if in.MergeStrategy != "source_thread" && in.MergeStrategy != "same_title" {
		return errors.New("invalid merge strategy")
	}
	if len([]rune(in.DisplayTitle)) > 255 || len([]rune(in.OperatorName)) > 64 {
		return errors.New("import task field too long")
	}
	if len(in.IdempotencyKey) > 191 {
		return errors.New("idempotency key too long")
	}
	return nil
}
func (s *Service) Create(ctx context.Context, in CreateInput) (Task, error) {
	in = normalize(in)
	if err := valid(in); err != nil {
		return Task{}, err
	}
	return s.Repo.Create(ctx, in)
}
func (s *Service) Retry(ctx context.Context, id int64, in CreateInput) (Task, error) {
	in = normalize(in)
	if id <= 0 {
		return Task{}, errors.New("invalid import task id")
	}
	if in.CandidateID == "" {
		in.CandidateID = "0"
	}
	return s.Repo.Retry(ctx, id, in)
}
func (s *Service) Cancel(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("invalid import task id")
	}
	return s.Repo.Cancel(ctx, id)
}
