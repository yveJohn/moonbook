package accountsync

import (
	"context"
	"testing"

	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	readercontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/contract"
)

type accountPagerStub struct {
	items []readercontract.Account
}

func (pager *accountPagerStub) AccountSnapshots(_ context.Context, afterID int64, limit int) (readercontract.AccountPage, error) {
	items := make([]readercontract.Account, 0, limit)
	more := false
	for _, item := range pager.items {
		if item.ID <= afterID {
			continue
		}
		if len(items) == limit {
			more = true
			break
		}
		items = append(items, item)
	}
	next := afterID
	if len(items) > 0 {
		next = items[len(items)-1].ID
	}
	return readercontract.AccountPage{Items: items, NextID: next, Done: !more}, nil
}

type projectionStoreStub struct {
	items   map[int64]commercecontract.ReaderSearchProjection
	upserts int
	deletes int
}

func (store *projectionStoreStub) ReaderSearchProjections(_ context.Context, afterID int64, limit int) (commercecontract.ReaderSearchProjectionPage, error) {
	ordered := make([]commercecontract.ReaderSearchProjection, 0, len(store.items))
	for id := afterID + 1; ; id++ {
		if item, ok := store.items[id]; ok {
			ordered = append(ordered, item)
			if len(ordered) == limit+1 {
				break
			}
		}
		if id > 10_000 {
			break
		}
	}
	more := len(ordered) > limit
	if more {
		ordered = ordered[:limit]
	}
	next := afterID
	if len(ordered) > 0 {
		next = ordered[len(ordered)-1].ReaderID
	}
	return commercecontract.ReaderSearchProjectionPage{Items: ordered, NextReaderID: next, Done: !more}, nil
}

func (store *projectionStoreStub) UpsertReaderSearchProjection(_ context.Context, item commercecontract.ReaderSearchProjection) error {
	store.upserts++
	store.items[item.ReaderID] = item
	return nil
}

func (store *projectionStoreStub) DeleteReaderSearchProjection(_ context.Context, readerID int64) error {
	store.deletes++
	delete(store.items, readerID)
	return nil
}

func TestCheckReportsDriftWithoutSensitiveAccountFields(t *testing.T) {
	accounts := &accountPagerStub{items: []readercontract.Account{
		{ID: 1, Username: "alpha@example.test", Nickname: "Alpha", Status: "enabled"},
		{ID: 2, Username: "beta@example.test", Nickname: "Beta", Status: "disabled"},
		{ID: 4, Username: "delta@example.test", Nickname: "Delta", Status: "enabled"},
	}}
	projections := &projectionStoreStub{items: map[int64]commercecontract.ReaderSearchProjection{
		1: {ReaderID: 1, Username: "alpha@example.test", Nickname: "Alpha", Status: "enabled"},
		2: {ReaderID: 2, Username: "stale@example.test", Nickname: "Beta", Status: "disabled"},
		3: {ReaderID: 3, Username: "extra@example.test", Nickname: "Extra", Status: "enabled"},
	}}
	service := NewService(accounts, projections, projections, 2)

	report, err := service.Check(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Missing != 1 || report.Mismatched != 1 || report.Extra != 1 || report.Repaired != 0 || len(report.Samples) != 3 {
		t.Fatalf("report=%+v", report)
	}
	for _, sample := range report.Samples {
		if sample.ReaderFingerprint == "" || sample.ReaderFingerprint == "1" || sample.ReaderFingerprint == "2" || sample.ReaderFingerprint == "3" || sample.ReaderFingerprint == "4" {
			t.Fatalf("reader ID was not fingerprinted: %+v", sample)
		}
		if sample.ReaderFingerprint == "alpha@example.test" || sample.ReaderFingerprint == "stale@example.test" {
			t.Fatalf("sample leaked account field: %+v", sample)
		}
	}
	if projections.upserts != 0 || projections.deletes != 0 {
		t.Fatalf("audit mode changed projections: upserts=%d deletes=%d", projections.upserts, projections.deletes)
	}
}

func TestCheckRepairConvergesProjection(t *testing.T) {
	accounts := &accountPagerStub{items: []readercontract.Account{
		{ID: 1, Username: "alpha", Nickname: "Alpha", Status: "enabled"},
		{ID: 2, Username: "beta", Nickname: "Beta", Status: "disabled"},
	}}
	projections := &projectionStoreStub{items: map[int64]commercecontract.ReaderSearchProjection{
		1: {ReaderID: 1, Username: "stale", Nickname: "Alpha", Status: "enabled"},
		3: {ReaderID: 3, Username: "extra", Nickname: "Extra", Status: "enabled"},
	}}
	service := NewService(accounts, projections, projections, 1)

	report, err := service.Check(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if report.Missing != 1 || report.Mismatched != 1 || report.Extra != 1 || report.Repaired != 3 || projections.upserts != 2 || projections.deletes != 1 {
		t.Fatalf("report=%+v upserts=%d deletes=%d", report, projections.upserts, projections.deletes)
	}
	clean, err := service.Check(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if clean.Missing != 0 || clean.Mismatched != 0 || clean.Extra != 0 {
		t.Fatalf("repair did not converge: %+v", clean)
	}
}
