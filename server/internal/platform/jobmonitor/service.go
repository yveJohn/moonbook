package jobmonitor

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const maxFilterLength = 128

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }

func (s *Service) List(ctx context.Context, filters Filters, page, size int) ([]Job, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	filters.Module = strings.TrimSpace(filters.Module)
	filters.JobType = strings.TrimSpace(filters.JobType)
	filters.Status = strings.TrimSpace(filters.Status)
	filters.LeaseOwner = strings.TrimSpace(filters.LeaseOwner)
	for name, value := range map[string]string{
		"module": filters.Module, "job type": filters.JobType, "lease owner": filters.LeaseOwner,
	} {
		if len(value) > maxFilterLength {
			return nil, 0, fmt.Errorf("%w: %s filter is too long", ErrInvalidArgument, name)
		}
	}
	if filters.Status != "" && !validStatus(filters.Status) {
		return nil, 0, fmt.Errorf("%w: invalid job status", ErrInvalidArgument)
	}
	from, err := parseTime(filters.From)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: invalid from time: %v", ErrInvalidArgument, err)
	}
	to, err := parseTime(filters.To)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: invalid to time: %v", ErrInvalidArgument, err)
	}
	if !from.IsZero() && !to.IsZero() && !from.Before(to) {
		return nil, 0, fmt.Errorf("%w: from time must be before to time", ErrInvalidArgument)
	}
	return s.Repo.List(ctx, filters, page, size)
}

func (s *Service) Get(ctx context.Context, id string) (Detail, error) {
	id = strings.TrimSpace(id)
	parsed, err := strconv.ParseInt(id, 10, 64)
	if err != nil || parsed <= 0 {
		return Detail{}, fmt.Errorf("%w: invalid job id", ErrInvalidArgument)
	}
	return s.Repo.Get(ctx, id)
}

func validStatus(status string) bool {
	switch status {
	case "pending", "running", "succeeded", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func parseTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, errors.New("must be RFC3339")
	}
	return parsed, nil
}

func sanitizeMessage(message string) string {
	message = strings.Join(strings.Fields(message), " ")
	if len(message) > 500 {
		message = message[:500] + "..."
	}
	lower := strings.ToLower(message)
	for _, marker := range []string{"password", "passwd", "api_key", "apikey", "authorization", "cookie", "secret", "token", "postgres://", "mysql://", "redis://", "s3://"} {
		if strings.Contains(lower, marker) {
			return "错误信息包含敏感字段，已脱敏"
		}
	}
	return message
}
