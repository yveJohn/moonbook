//go:build integration

package readersearch

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

func TestSQLRepositoryUpsertPageAndDelete(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repository := SQLRepository{DB: db}
	service := NewService(repository)
	baseID := time.Now().UnixNano()
	if baseID < 0 {
		baseID = -baseID
	}

	readerIDs := []int64{baseID, baseID + 1, baseID + 2}
	t.Cleanup(func() {
		for _, readerID := range readerIDs {
			_, _ = db.ExecContext(context.Background(), `DELETE FROM commerce_reader_search_projection WHERE reader_id=$1`, readerID)
		}
	})
	for index, readerID := range readerIDs {
		if err := service.Upsert(ctx, Projection{
			ReaderID: readerID,
			Username: fmt.Sprintf("projection-%d", readerID),
			Nickname: fmt.Sprintf("Reader %d", index),
			Status:   "enabled",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.Upsert(ctx, Projection{ReaderID: readerIDs[0], Username: "projection-updated", Nickname: "Updated", Status: "disabled"}); err != nil {
		t.Fatal(err)
	}

	first, err := service.Page(ctx, baseID-1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || first.Done || first.NextReaderID != readerIDs[1] || first.Items[0].Username != "projection-updated" {
		t.Fatalf("first page = %+v", first)
	}
	second, err := service.Page(ctx, first.NextReaderID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || !second.Done || second.Items[0].ReaderID != readerIDs[2] {
		t.Fatalf("second page = %+v", second)
	}

	if err := service.Delete(ctx, readerIDs[2]); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM commerce_reader_search_projection WHERE reader_id=$1`, readerIDs[2]).Scan(&count); err != nil || count != 0 {
		t.Fatalf("deleted count = %d, err = %v", count, err)
	}
}

func TestSQLRepositoryJoinsContextTransaction(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	service := NewService(SQLRepository{DB: db})
	readerID := time.Now().UnixNano()
	if readerID < 0 {
		readerID = -readerID
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM commerce_reader_search_projection WHERE reader_id=$1`, readerID)
	})

	wantRollback := fmt.Errorf("force rollback")
	err := transaction.New(db).Within(ctx, func(txCtx context.Context) error {
		if err := service.Upsert(txCtx, Projection{ReaderID: readerID, Username: "transactional-projection", Status: "enabled"}); err != nil {
			return err
		}
		return wantRollback
	})
	if err == nil {
		t.Fatal("expected rollback error")
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM commerce_reader_search_projection WHERE reader_id=$1`, readerID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rolled back count = %d, err = %v", count, err)
	}
}
