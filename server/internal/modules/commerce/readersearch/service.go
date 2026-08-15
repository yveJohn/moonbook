package readersearch

import (
	"context"
	"strings"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (service *Service) Upsert(ctx context.Context, projection Projection) error {
	projection.Username = strings.TrimSpace(projection.Username)
	projection.Nickname = strings.TrimSpace(projection.Nickname)
	projection.Status = strings.TrimSpace(projection.Status)
	if projection.ReaderID <= 0 || projection.Username == "" || len([]rune(projection.Username)) > 64 || len([]rune(projection.Nickname)) > 64 || !validStatus(projection.Status) {
		return ErrInvalidProjection
	}
	if service == nil || service.repository == nil {
		return ErrRepositoryMissing
	}
	return service.repository.Upsert(ctx, projection)
}

func (service *Service) Delete(ctx context.Context, readerID int64) error {
	if readerID <= 0 {
		return ErrInvalidProjection
	}
	if service == nil || service.repository == nil {
		return ErrRepositoryMissing
	}
	return service.repository.Delete(ctx, readerID)
}

func (service *Service) Page(ctx context.Context, afterReaderID int64, limit int) (Page, error) {
	if afterReaderID < 0 || limit <= 0 || limit > MaxPageSize {
		return Page{}, ErrInvalidPage
	}
	if service == nil || service.repository == nil {
		return Page{}, ErrRepositoryMissing
	}
	return service.repository.Page(ctx, afterReaderID, limit)
}

func validStatus(status string) bool {
	return status == "enabled" || status == "disabled" || status == "deleted"
}
