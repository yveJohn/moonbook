package provider

import (
	"context"
	"database/sql"
	"errors"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/readersearch"
)

type ReaderSearch struct {
	service *readersearch.Service
}

var (
	_ contract.ReaderSearchProjectionWriter = (*ReaderSearch)(nil)
	_ contract.ReaderSearchProjectionReader = (*ReaderSearch)(nil)
)

func NewReaderSearch(db *sql.DB) *ReaderSearch {
	return &ReaderSearch{service: readersearch.NewService(readersearch.SQLRepository{DB: db})}
}

func NewReaderSearchWithService(service *readersearch.Service) *ReaderSearch {
	return &ReaderSearch{service: service}
}

func (provider *ReaderSearch) UpsertReaderSearchProjection(ctx context.Context, projection contract.ReaderSearchProjection) error {
	err := provider.service.Upsert(ctx, readersearch.Projection{
		ReaderID: projection.ReaderID,
		Username: projection.Username,
		Nickname: projection.Nickname,
		Status:   projection.Status,
	})
	return wrapReaderSearchError(err)
}

func (provider *ReaderSearch) DeleteReaderSearchProjection(ctx context.Context, readerID int64) error {
	return wrapReaderSearchError(provider.service.Delete(ctx, readerID))
}

func (provider *ReaderSearch) ReaderSearchProjections(ctx context.Context, afterReaderID int64, limit int) (contract.ReaderSearchProjectionPage, error) {
	page, err := provider.service.Page(ctx, afterReaderID, limit)
	if err != nil {
		return contract.ReaderSearchProjectionPage{}, wrapReaderSearchError(err)
	}
	items := make([]contract.ReaderSearchProjection, 0, len(page.Items))
	for _, projection := range page.Items {
		items = append(items, contract.ReaderSearchProjection{
			ReaderID: projection.ReaderID,
			Username: projection.Username,
			Nickname: projection.Nickname,
			Status:   projection.Status,
		})
	}
	return contract.ReaderSearchProjectionPage{
		Items:        items,
		NextReaderID: page.NextReaderID,
		Done:         page.Done,
	}, nil
}

func wrapReaderSearchError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, readersearch.ErrInvalidProjection), errors.Is(err, readersearch.ErrInvalidPage):
		return contract.Wrap(contract.ErrInvalidRequest, err)
	case errors.Is(err, context.DeadlineExceeded):
		return contract.Wrap(contract.ErrTimeout, err)
	default:
		return contract.Wrap(contract.ErrUnavailable, err)
	}
}
