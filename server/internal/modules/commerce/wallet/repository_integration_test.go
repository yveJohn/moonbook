//go:build integration

package wallet

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestGetReleasesSingleConnection(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	readerID := time.Now().UnixNano()
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'x','enabled')`, readerID, "wallet-pool-"+integrationtest.Prefix()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		for _, query := range []string{`DELETE FROM reader_wallets WHERE reader_id=$1`, `DELETE FROM reader_accounts WHERE id=$1`} {
			if _, err := db.ExecContext(cleanup, query, readerID); err != nil {
				t.Error(err)
			}
		}
	})
	repository := SQLRepository{DB: db}
	// Keep the context alive across calls: cancellation must not mask a leak.
	for i := 0; i < 100; i++ {
		w, err := repository.Get(ctx, readerID)
		if err != nil {
			t.Fatalf("Get iteration %d: %v", i, err)
		}
		if w.ReaderID != readerID || w.RechargeCoinBalance != 0 || w.BonusCoinBalance != 0 {
			t.Fatalf("unexpected wallet: %+v", w)
		}
		if stats := db.Stats(); stats.InUse != 0 {
			t.Fatalf("connection retained after Get: %+v", stats)
		}
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10 {
				if _, err := repository.Get(ctx, readerID); err != nil {
					t.Errorf("concurrent Get: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if stats := db.Stats(); stats.InUse != 0 {
		t.Fatalf("connection retained after concurrent Get: %+v", stats)
	}
	// A failed initialization must also release its connection.
	if _, err := repository.Get(ctx, -readerID); err == nil {
		t.Fatal("expected foreign-key error for missing reader")
	}
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("pool unusable after failed initialization: %v", err)
	}
}
