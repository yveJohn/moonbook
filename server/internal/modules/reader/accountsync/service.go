package accountsync

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	readercontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

const (
	defaultPageSize = 500
	maxPageSize     = 1000
	maxSamples      = 100
)

type DriftSample struct {
	Kind              string
	ReaderFingerprint string
}

type Report struct {
	Missing    int64
	Mismatched int64
	Extra      int64
	Repaired   int64
	Samples    []DriftSample
}

type Service struct {
	accounts    readercontract.AccountSnapshotPager
	projections commercecontract.ReaderSearchProjectionReader
	writer      commercecontract.ReaderSearchProjectionWriter
	pageSize    int
}

func NewService(
	accounts readercontract.AccountSnapshotPager,
	projections commercecontract.ReaderSearchProjectionReader,
	writer commercecontract.ReaderSearchProjectionWriter,
	pageSize int,
) *Service {
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return &Service{accounts: accounts, projections: projections, writer: writer, pageSize: pageSize}
}

func (service *Service) Check(ctx context.Context, repair bool) (Report, error) {
	if service == nil || service.accounts == nil || service.projections == nil || repair && service.writer == nil {
		return Report{}, errors.New("reader account projection consistency service is unavailable")
	}
	accounts := accountIterator{source: service.accounts, limit: service.pageSize}
	projections := projectionIterator{source: service.projections, limit: service.pageSize}
	report := Report{Samples: make([]DriftSample, 0)}

	for {
		account, err := accounts.peek(ctx)
		if err != nil {
			return Report{}, err
		}
		projection, err := projections.peek(ctx)
		if err != nil {
			return Report{}, err
		}
		if account == nil && projection == nil {
			return report, nil
		}

		switch {
		case projection == nil || account != nil && account.ID < projection.ReaderID:
			report.Missing++
			report.addSample("missing", account.ID)
			if repair {
				if err := service.writer.UpsertReaderSearchProjection(ctx, toProjection(*account)); err != nil {
					return Report{}, err
				}
				report.Repaired++
			}
			accounts.consume()
		case account == nil || projection.ReaderID < account.ID:
			report.Extra++
			report.addSample("extra", projection.ReaderID)
			if repair {
				if err := service.writer.DeleteReaderSearchProjection(ctx, projection.ReaderID); err != nil {
					return Report{}, err
				}
				report.Repaired++
			}
			projections.consume()
		default:
			if !sameProjection(*account, *projection) {
				report.Mismatched++
				report.addSample("mismatch", account.ID)
				if repair {
					if err := service.writer.UpsertReaderSearchProjection(ctx, toProjection(*account)); err != nil {
						return Report{}, err
					}
					report.Repaired++
				}
			}
			accounts.consume()
			projections.consume()
		}
	}
}

func (report *Report) addSample(kind string, readerID int64) {
	if len(report.Samples) >= maxSamples {
		return
	}
	sum := sha256.Sum256([]byte("moonbook-reader-account:" + strconv.FormatInt(readerID, 10)))
	report.Samples = append(report.Samples, DriftSample{Kind: kind, ReaderFingerprint: hex.EncodeToString(sum[:8])})
}

func toProjection(account readercontract.Account) commercecontract.ReaderSearchProjection {
	return commercecontract.ReaderSearchProjection{
		ReaderID: account.ID,
		Username: account.Username,
		Nickname: account.Nickname,
		Status:   account.Status,
	}
}

func sameProjection(account readercontract.Account, projection commercecontract.ReaderSearchProjection) bool {
	return account.ID == projection.ReaderID &&
		account.Username == projection.Username &&
		account.Nickname == projection.Nickname &&
		account.Status == projection.Status
}

type accountIterator struct {
	source      readercontract.AccountSnapshotPager
	limit       int
	cursor      int64
	items       []readercontract.Account
	index       int
	done        bool
	initialized bool
}

func (iterator *accountIterator) peek(ctx context.Context) (*readercontract.Account, error) {
	for iterator.index >= len(iterator.items) {
		if iterator.initialized && iterator.done {
			return nil, nil
		}
		page, err := iterator.source.AccountSnapshots(ctx, iterator.cursor, iterator.limit)
		if err != nil {
			return nil, err
		}
		if err := validateAccountsPage(iterator.cursor, page); err != nil {
			return nil, err
		}
		iterator.items = page.Items
		iterator.index = 0
		iterator.done = page.Done
		iterator.initialized = true
		iterator.cursor = page.NextID
	}
	return &iterator.items[iterator.index], nil
}

func (iterator *accountIterator) consume() { iterator.index++ }

type projectionIterator struct {
	source      commercecontract.ReaderSearchProjectionReader
	limit       int
	cursor      int64
	items       []commercecontract.ReaderSearchProjection
	index       int
	done        bool
	initialized bool
}

func (iterator *projectionIterator) peek(ctx context.Context) (*commercecontract.ReaderSearchProjection, error) {
	for iterator.index >= len(iterator.items) {
		if iterator.initialized && iterator.done {
			return nil, nil
		}
		page, err := iterator.source.ReaderSearchProjections(ctx, iterator.cursor, iterator.limit)
		if err != nil {
			return nil, err
		}
		if err := validateProjectionPage(iterator.cursor, page); err != nil {
			return nil, err
		}
		iterator.items = page.Items
		iterator.index = 0
		iterator.done = page.Done
		iterator.initialized = true
		iterator.cursor = page.NextReaderID
	}
	return &iterator.items[iterator.index], nil
}

func (iterator *projectionIterator) consume() { iterator.index++ }

func validateAccountsPage(cursor int64, page readercontract.AccountPage) error {
	previous := cursor
	for _, item := range page.Items {
		if item.ID <= previous {
			return errors.New("reader account page is not strictly ordered")
		}
		previous = item.ID
	}
	if len(page.Items) > 0 && page.NextID != previous {
		return errors.New("reader account page cursor does not match its last item")
	}
	if !page.Done && (len(page.Items) == 0 || page.NextID <= cursor) {
		return errors.New("reader account pager made no progress")
	}
	return nil
}

func validateProjectionPage(cursor int64, page commercecontract.ReaderSearchProjectionPage) error {
	previous := cursor
	for _, item := range page.Items {
		if item.ReaderID <= previous {
			return errors.New("commerce reader projection page is not strictly ordered")
		}
		previous = item.ReaderID
	}
	if len(page.Items) > 0 && page.NextReaderID != previous {
		return errors.New("commerce reader projection page cursor does not match its last item")
	}
	if !page.Done && (len(page.Items) == 0 || page.NextReaderID <= cursor) {
		return errors.New("commerce reader projection pager made no progress")
	}
	return nil
}

type SQLAccountPager struct {
	DB *sql.DB
}

var _ readercontract.AccountSnapshotPager = SQLAccountPager{}

func (pager SQLAccountPager) AccountSnapshots(ctx context.Context, afterID int64, limit int) (readercontract.AccountPage, error) {
	if pager.DB == nil || afterID < 0 || limit <= 0 || limit > maxPageSize {
		return readercontract.AccountPage{}, errors.New("invalid reader account page")
	}
	rows, err := transaction.Executor(ctx, pager.DB).QueryContext(ctx, `
SELECT id, username, nickname, status
FROM reader_accounts
WHERE id > $1
ORDER BY id
LIMIT $2`, afterID, limit+1)
	if err != nil {
		return readercontract.AccountPage{}, fmt.Errorf("read reader account snapshots: %w", err)
	}
	defer rows.Close()

	items := make([]readercontract.Account, 0, limit+1)
	for rows.Next() {
		var item readercontract.Account
		if err := rows.Scan(&item.ID, &item.Username, &item.Nickname, &item.Status); err != nil {
			return readercontract.AccountPage{}, fmt.Errorf("scan reader account snapshot: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return readercontract.AccountPage{}, fmt.Errorf("iterate reader account snapshots: %w", err)
	}

	done := len(items) <= limit
	if !done {
		items = items[:limit]
	}
	nextID := afterID
	if len(items) > 0 {
		nextID = items[len(items)-1].ID
	}
	return readercontract.AccountPage{Items: items, NextID: nextID, Done: done}, nil
}
