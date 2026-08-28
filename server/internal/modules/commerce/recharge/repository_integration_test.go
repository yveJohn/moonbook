//go:build integration

package recharge

import (
	"context"
	"database/sql"
	"strconv"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/epusdt"
)

func TestRechargeCatalogQuoteAndOrderStateMachine(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	readerID, productID := base, base+1
	username := "recharge-it-" + integrationtest.Prefix()
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'x','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_recharge_products(id,product_name,diamond_amount,price_usdt,sale_status,sort_order) VALUES($1,'集成充值',17,2.00,'on_sale',1)`, productID); err != nil {
		t.Fatal(err)
	}
	if err := integrationtest.EnablePaymentChannel(ctx, db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_recharge_orders WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_recharge_products WHERE id=$1`, productID)
		_, _ = db.ExecContext(context.Background(), `UPDATE reader_payment_channels SET enabled=false WHERE provider='epusdt'`)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})

	r := SQLRepository{DB: db}
	catalog, err := r.Catalog(ctx)
	if err != nil || len(catalog.Products) == 0 {
		t.Fatalf("catalog=%+v err=%v", catalog, err)
	}
	quote, err := r.Quote(ctx, 17)
	if err != nil || quote.PriceUSDT != "2.43" {
		t.Fatalf("quote=%+v err=%v", quote, err)
	}
	prepared, err := r.Prepare(ctx, CreateRequest{ReaderID: readerID, ProductID: &productID, RequestID: "request-" + integrationtest.Prefix()})
	if err != nil {
		t.Fatal(err)
	}
	prepared.CredentialRef = "primary"
	prepared.MerchantPIDSnapshot = "merchant"
	first, err := r.Start(ctx, prepared)
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.Start(ctx, prepared)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created || second.Created || first.Order.ID != second.Order.ID || first.Order.OrderNo != second.Order.OrderNo || first.Order.Status != "creating" {
		t.Fatalf("idempotency first=%+v second=%+v", first, second)
	}
	if len(first.Order.OrderNo) > 32 || first.Order.OrderNo != "RC"+first.Order.ID {
		t.Fatalf("unexpected order number: %+v", first.Order)
	}
	completed, err := r.Complete(ctx, first.Order.ID, epusdt.CreateResponse{
		TradeID: "trade-" + integrationtest.Prefix(), OrderID: first.Order.OrderNo, Amount: first.Order.PriceUSDT,
		ActualAmount: "2.43000000", ReceiveAddress: "test-address", PaymentURL: "https://pay.example/order", Status: 1,
		ExpirationTime: time.Now().Add(20 * time.Minute).UTC(),
	})
	if err != nil || completed.Status != "pending" || completed.GatewayTradeID == nil || completed.ActualAmount == nil || completed.ReceiveAddress == nil || completed.PaymentURL == nil || completed.GatewayStatus == nil || completed.ExpireTime == nil {
		t.Fatalf("completed=%+v err=%v", completed, err)
	}
	got, err := r.GetOrder(ctx, readerID, first.Order.ID)
	if err != nil || got.ReaderID != first.Order.ReaderID {
		t.Fatalf("order=%+v err=%v", got, err)
	}
}

func TestRechargeStartReplacesPendingAndProtectsUnknownOrder(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	readerID := time.Now().UnixNano()
	username := "recharge-state-" + integrationtest.Prefix()
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'x','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	var channelWasEnabled bool
	if err := db.QueryRowContext(ctx, `SELECT enabled FROM reader_payment_channels WHERE provider='epusdt'`).Scan(&channelWasEnabled); err != nil {
		t.Fatal(err)
	}
	if err := integrationtest.EnablePaymentChannel(ctx, db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_recharge_orders WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `UPDATE reader_payment_channels SET enabled=$1 WHERE provider='epusdt'`, channelWasEnabled)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})

	repository := SQLRepository{DB: db}
	prepare := func(requestID string) PreparedOrder {
		prepared, err := repository.Prepare(ctx, CreateRequest{ReaderID: readerID, CustomDiamondAmount: "100", RequestID: requestID})
		if err != nil {
			t.Fatal(err)
		}
		prepared.CredentialRef = "primary"
		prepared.MerchantPIDSnapshot = "merchant"
		return prepared
	}
	first, err := repository.Start(ctx, prepare("first-"+integrationtest.Prefix()))
	if err != nil || !first.Created {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	creatingBlocked, err := repository.Start(ctx, prepare("creating-blocked-"+integrationtest.Prefix()))
	if err != nil || creatingBlocked.Created || creatingBlocked.Order.ID != first.Order.ID || creatingBlocked.Order.Status != "creating" {
		t.Fatalf("creating blocked=%+v err=%v", creatingBlocked, err)
	}
	firstOrder, err := repository.Complete(ctx, first.Order.ID, epusdt.CreateResponse{
		TradeID: "replace-trade-" + integrationtest.Prefix(), ActualAmount: "1.00000000", ReceiveAddress: "address",
		PaymentURL: "https://pay.example/first", Status: 1, ExpirationTime: time.Now().Add(20 * time.Minute),
	})
	if err != nil || firstOrder.Status != "pending" {
		t.Fatalf("first order=%+v err=%v", firstOrder, err)
	}

	second, err := repository.Start(ctx, prepare("second-"+integrationtest.Prefix()))
	if err != nil || !second.Created || second.Order.Status != "creating" {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	var oldStatus string
	var oldActive sql.NullInt64
	var oldFailure sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT status,active_reader_id,failure_code FROM reader_recharge_orders WHERE id=$1`, first.Order.ID).Scan(&oldStatus, &oldActive, &oldFailure); err != nil {
		t.Fatal(err)
	}
	if oldStatus != "superseded" || oldActive.Valid || !oldFailure.Valid || oldFailure.String != "ORDER_REPLACED" {
		t.Fatalf("old status=%s active=%+v failure=%+v", oldStatus, oldActive, oldFailure)
	}

	unknown, err := repository.Fail(ctx, second.Order.ID, &epusdt.GatewayError{Code: "REQUEST_TIMEOUT", Class: epusdt.FailureUncertain, Summary: "timed out"})
	if err != nil || unknown.Status != "gateway_unknown" {
		t.Fatalf("unknown=%+v err=%v", unknown, err)
	}
	blockedPrepared := prepare("blocked-" + integrationtest.Prefix())
	blocked, err := repository.Start(ctx, blockedPrepared)
	if err != nil || blocked.Created || blocked.Order.ID != second.Order.ID || blocked.Order.Status != "gateway_unknown" {
		t.Fatalf("blocked=%+v err=%v", blocked, err)
	}

	if _, err := db.ExecContext(ctx, `UPDATE reader_recharge_orders SET expire_time=now()-interval '1 second' WHERE id=$1`, second.Order.ID); err != nil {
		t.Fatal(err)
	}
	afterExpiry, err := repository.Start(ctx, blockedPrepared)
	if err != nil || !afterExpiry.Created || afterExpiry.Order.ID == second.Order.ID {
		t.Fatalf("after expiry=%+v err=%v", afterExpiry, err)
	}
	var expiredStatus string
	var expiredActive sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT status,active_reader_id FROM reader_recharge_orders WHERE id=$1`, second.Order.ID).Scan(&expiredStatus, &expiredActive); err != nil {
		t.Fatal(err)
	}
	if expiredStatus != "expired" || expiredActive.Valid {
		t.Fatalf("expired status=%s active=%+v", expiredStatus, expiredActive)
	}
	legacyTradeID := "paid-lock-trade-" + integrationtest.Prefix()
	if _, err := db.ExecContext(ctx, `UPDATE reader_recharge_orders SET status='paid',gateway_trade_id=$1,actual_amount='1.00000000',paid_time=now() WHERE id=$2`, legacyTradeID, afterExpiry.Order.ID); err != nil {
		t.Fatal(err)
	}
	healed, err := repository.Start(ctx, prepare("after-paid-lock-"+integrationtest.Prefix()))
	if err != nil || !healed.Created || healed.Order.ID == afterExpiry.Order.ID {
		t.Fatalf("healed=%+v err=%v", healed, err)
	}
	var paidStatus, paidTradeID, paidActualAmount string
	var paidActive sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT status,active_reader_id,gateway_trade_id,actual_amount::text FROM reader_recharge_orders WHERE id=$1`, afterExpiry.Order.ID).Scan(&paidStatus, &paidActive, &paidTradeID, &paidActualAmount); err != nil {
		t.Fatal(err)
	}
	if paidStatus != "paid" || paidActive.Valid || paidTradeID != legacyTradeID || paidActualAmount != "1.00000000" {
		t.Fatalf("paid status=%s active=%+v trade=%q actual=%q", paidStatus, paidActive, paidTradeID, paidActualAmount)
	}
	failed, err := repository.Fail(ctx, healed.Order.ID, &epusdt.GatewayError{Code: "GATEWAY_REJECTED", Class: epusdt.FailureDefinite, Summary: "rejected"})
	if err != nil || failed.Status != "create_failed" {
		t.Fatalf("failed=%+v err=%v", failed, err)
	}
	var active sql.NullInt64
	var credentialRef, merchantPID string
	if err := db.QueryRowContext(ctx, `SELECT active_reader_id,credential_ref,merchant_pid_snapshot FROM reader_recharge_orders WHERE id=$1`, healed.Order.ID).Scan(&active, &credentialRef, &merchantPID); err != nil {
		t.Fatal(err)
	}
	if active.Valid || credentialRef != "primary" || merchantPID != "merchant" {
		t.Fatalf("active=%+v credential=%q merchant=%q", active, credentialRef, merchantPID)
	}
}

func TestRechargeStartWaitsForReaderAdvisoryLock(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	readerID := time.Now().UnixNano()
	username := "recharge-lock-" + integrationtest.Prefix()
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'x','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	var channelWasEnabled bool
	if err := db.QueryRowContext(ctx, `SELECT enabled FROM reader_payment_channels WHERE provider='epusdt'`).Scan(&channelWasEnabled); err != nil {
		t.Fatal(err)
	}
	if err := integrationtest.EnablePaymentChannel(ctx, db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_recharge_orders WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `UPDATE reader_payment_channels SET enabled=$1 WHERE provider='epusdt'`, channelWasEnabled)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})
	repository := SQLRepository{DB: db}
	prepared, err := repository.Prepare(ctx, CreateRequest{ReaderID: readerID, CustomDiamondAmount: "100", RequestID: "lock-" + integrationtest.Prefix()})
	if err != nil {
		t.Fatal(err)
	}
	prepared.CredentialRef = "primary"
	prepared.MerchantPIDSnapshot = "merchant"

	locker, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := locker.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, readerID); err != nil {
		_ = locker.Rollback()
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, startErr := repository.Start(ctx, prepared)
		result <- startErr
	}()
	select {
	case err := <-result:
		_ = locker.Rollback()
		t.Fatalf("Start returned before advisory lock release: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if err := locker.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not continue after advisory lock release")
	}
}

func TestRechargeStartRereadsSameRequestAfterUniqueConflict(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	readerID := time.Now().UnixNano()
	requestID := "conflict-" + integrationtest.Prefix()
	username := "recharge-conflict-" + integrationtest.Prefix()
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'x','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	var channelWasEnabled bool
	if err := db.QueryRowContext(ctx, `SELECT enabled FROM reader_payment_channels WHERE provider='epusdt'`).Scan(&channelWasEnabled); err != nil {
		t.Fatal(err)
	}
	if err := integrationtest.EnablePaymentChannel(ctx, db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_recharge_orders WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `UPDATE reader_payment_channels SET enabled=$1 WHERE provider='epusdt'`, channelWasEnabled)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})

	repository := SQLRepository{DB: db}
	prepared, err := repository.Prepare(ctx, CreateRequest{ReaderID: readerID, CustomDiamondAmount: "100", RequestID: requestID})
	if err != nil {
		t.Fatal(err)
	}
	prepared.CredentialRef = "primary"
	prepared.MerchantPIDSnapshot = "merchant"

	conflicting, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var expectedID int64
	if err := conflicting.QueryRowContext(ctx, `INSERT INTO reader_recharge_orders(order_no,reader_id,request_id,source_type,diamond_amount,price_usdt,provider,currency,token,network,status) VALUES($1,$2,$3,'custom',100,'1.00','epusdt','usd','usdt','tron','create_failed') RETURNING id`, "CONFLICT-"+integrationtest.Prefix(), readerID, requestID).Scan(&expectedID); err != nil {
		_ = conflicting.Rollback()
		t.Fatal(err)
	}
	type startOutcome struct {
		result StartResult
		err    error
	}
	outcome := make(chan startOutcome, 1)
	go func() {
		result, startErr := repository.Start(ctx, prepared)
		outcome <- startOutcome{result: result, err: startErr}
	}()
	select {
	case early := <-outcome:
		_ = conflicting.Rollback()
		t.Fatalf("Start returned before conflicting transaction completed: %+v", early)
	case <-time.After(150 * time.Millisecond):
	}
	if err := conflicting.Commit(); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-outcome:
		if got.err != nil || got.result.Created || got.result.Order.ID != strconv.FormatInt(expectedID, 10) || got.result.Order.Status != "create_failed" {
			t.Fatalf("result=%+v err=%v expectedID=%d", got.result, got.err, expectedID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not resolve the unique conflict")
	}
}
