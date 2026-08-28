//go:build integration

package recharge

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

type expiryFixture struct {
	db      *sql.DB
	readers map[string]int64
	now     time.Time
}

func newExpiryFixture(t *testing.T) expiryFixture {
	t.Helper()
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	now := time.Now().UTC().Truncate(time.Second)
	readers := make(map[string]int64)
	for index, name := range []string{"pending-past", "pending-future", "unknown-old", "unknown-trade", "creating-old", "creating-fresh", "paid", "failed", "lazy-create", "lazy-get"} {
		readerID := time.Now().UnixNano() + int64(index)
		readers[name] = readerID
		if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'x','enabled')`, readerID, "expiry-"+name+"-"+integrationtest.Prefix()); err != nil {
			t.Fatal(err)
		}
	}
	var channelWasEnabled bool
	if err := db.QueryRowContext(ctx, `SELECT enabled FROM reader_payment_channels WHERE provider='epusdt'`).Scan(&channelWasEnabled); err != nil {
		t.Fatal(err)
	}
	if err := integrationtest.EnablePaymentChannel(ctx, db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, readerID := range readers {
			_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_recharge_orders WHERE reader_id=$1`, readerID)
			_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID)
		}
		_, _ = db.ExecContext(context.Background(), `UPDATE reader_payment_channels SET enabled=$1 WHERE provider='epusdt'`, channelWasEnabled)
	})
	return expiryFixture{db: db, readers: readers, now: now}
}

func (fixture expiryFixture) insertOrder(t *testing.T, name, status string, createdAt time.Time, expireTime *time.Time, gatewayTradeID string) string {
	t.Helper()
	readerID := fixture.readers[name]
	var active any
	if status == "creating" || status == "pending" || status == "gateway_unknown" {
		active = readerID
	}
	var id string
	err := fixture.db.QueryRow(`INSERT INTO reader_recharge_orders(order_no,reader_id,request_id,source_type,diamond_amount,price_usdt,provider,currency,token,network,gateway_trade_id,status,expire_time,active_reader_id,created_at,updated_at) VALUES($1,$2,$3,'custom',100,'1.00','epusdt','usd','usdt','tron',NULLIF($4,''),$5,$6,$7,$8,$8) RETURNING id::text`,
		"EXP-"+name+"-"+integrationtest.Prefix(), readerID, "request-"+name, gatewayTradeID, status, expireTime, active, createdAt).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestExpireBatchAppliesStatusSpecificDeadlinesAndBatchLimit(t *testing.T) {
	fixture := newExpiryFixture(t)
	past, future := fixture.now.Add(-time.Minute), fixture.now.Add(time.Minute)
	ids := map[string]string{
		"pending-past":   fixture.insertOrder(t, "pending-past", "pending", fixture.now.Add(-time.Hour), &past, "trade-pending"),
		"pending-future": fixture.insertOrder(t, "pending-future", "pending", fixture.now.Add(-time.Hour), &future, "trade-future"),
		"unknown-old":    fixture.insertOrder(t, "unknown-old", "gateway_unknown", fixture.now.Add(-20*time.Minute), nil, ""),
		"unknown-trade":  fixture.insertOrder(t, "unknown-trade", "gateway_unknown", fixture.now.Add(-20*time.Minute), nil, "trade-known"),
		"creating-old":   fixture.insertOrder(t, "creating-old", "creating", fixture.now.Add(-20*time.Minute), nil, ""),
		"creating-fresh": fixture.insertOrder(t, "creating-fresh", "creating", fixture.now.Add(-5*time.Minute), nil, ""),
		"paid":           fixture.insertOrder(t, "paid", "paid", fixture.now.Add(-time.Hour), &past, "trade-paid"),
		"failed":         fixture.insertOrder(t, "failed", "create_failed", fixture.now.Add(-time.Hour), &past, ""),
	}
	repository := SQLRepository{DB: fixture.db, UnknownReleaseWindow: 15 * time.Minute, Now: func() time.Time { return fixture.now }}

	first, err := repository.ExpireBatch(context.Background(), fixture.now, 15*time.Minute, 2)
	if err != nil || first != 2 {
		t.Fatalf("first=%d err=%v", first, err)
	}
	second, err := repository.ExpireBatch(context.Background(), fixture.now, 15*time.Minute, 2)
	if err != nil || second != 1 {
		t.Fatalf("second=%d err=%v", second, err)
	}
	third, err := repository.ExpireBatch(context.Background(), fixture.now, 15*time.Minute, 2)
	if err != nil || third != 0 {
		t.Fatalf("third=%d err=%v", third, err)
	}

	for name, id := range ids {
		var status string
		var active sql.NullInt64
		var failure sql.NullString
		if err := fixture.db.QueryRow(`SELECT status,active_reader_id,failure_code FROM reader_recharge_orders WHERE id=$1`, id).Scan(&status, &active, &failure); err != nil {
			t.Fatal(err)
		}
		wantExpired := name == "pending-past" || name == "unknown-old" || name == "creating-old"
		if wantExpired && (status != "expired" || active.Valid || !failure.Valid || failure.String != "ORDER_EXPIRED") {
			t.Fatalf("%s status=%s active=%+v failure=%+v", name, status, active, failure)
		}
		if !wantExpired {
			wantStatus := map[string]string{"pending-future": "pending", "unknown-trade": "gateway_unknown", "creating-fresh": "creating", "paid": "paid", "failed": "create_failed"}[name]
			if status != wantStatus {
				t.Fatalf("%s status=%s want=%s", name, status, wantStatus)
			}
		}
	}
}

func TestLazyExpiryReleasesReaderForCreateAndUpdatesGet(t *testing.T) {
	fixture := newExpiryFixture(t)
	window := 15 * time.Minute
	repository := SQLRepository{DB: fixture.db, UnknownReleaseWindow: window, Now: func() time.Time { return fixture.now }}
	oldCreating := fixture.insertOrder(t, "lazy-create", "creating", fixture.now.Add(-20*time.Minute), nil, "")
	retry, err := repository.Prepare(context.Background(), CreateRequest{ReaderID: fixture.readers["lazy-create"], CustomDiamondAmount: "100", RequestID: "request-lazy-create"})
	if err != nil {
		t.Fatal(err)
	}
	retry.CredentialRef, retry.MerchantPIDSnapshot = "primary", "merchant"
	retried, err := repository.Start(context.Background(), retry)
	if err != nil || retried.Created || retried.Order.ID != oldCreating || retried.Order.Status != "expired" {
		t.Fatalf("retried=%+v err=%v", retried, err)
	}

	prepared, err := repository.Prepare(context.Background(), CreateRequest{ReaderID: fixture.readers["lazy-create"], CustomDiamondAmount: "100", RequestID: "replacement"})
	if err != nil {
		t.Fatal(err)
	}
	prepared.CredentialRef, prepared.MerchantPIDSnapshot = "primary", "merchant"
	started, err := repository.Start(context.Background(), prepared)
	if err != nil || !started.Created || started.Order.ID == oldCreating {
		t.Fatalf("started=%+v err=%v", started, err)
	}
	assertExpiryState(t, fixture.db, oldCreating, "expired", false)

	past := fixture.now.Add(-time.Minute)
	oldPending := fixture.insertOrder(t, "lazy-get", "pending", fixture.now.Add(-time.Hour), &past, "trade-lazy-get")
	got, err := repository.GetOrder(context.Background(), fixture.readers["lazy-get"], oldPending)
	if err != nil || got.Status != "expired" {
		t.Fatalf("order=%+v err=%v", got, err)
	}
	assertExpiryState(t, fixture.db, oldPending, "expired", false)
}

func assertExpiryState(t *testing.T, db *sql.DB, id, wantStatus string, wantActive bool) {
	t.Helper()
	var status string
	var active sql.NullInt64
	if err := db.QueryRow(`SELECT status,active_reader_id FROM reader_recharge_orders WHERE id=$1`, id).Scan(&status, &active); err != nil {
		t.Fatal(err)
	}
	if status != wantStatus || active.Valid != wantActive {
		t.Fatalf("order %s status=%s active=%+v want=%s/%t", fmt.Sprint(id), status, active, wantStatus, wantActive)
	}
}
