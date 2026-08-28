//go:build integration

package recharge

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/epusdt"
)

type integrationGateway struct {
	calls        atomic.Int32
	started      chan struct{}
	release      chan struct{}
	failure      *epusdt.GatewayError
	beforeReturn func()
}

func (gateway *integrationGateway) Create(ctx context.Context, request epusdt.CreateRequest) (epusdt.CreateResponse, error) {
	if gateway.calls.Add(1) == 1 && gateway.started != nil {
		close(gateway.started)
	}
	if gateway.release != nil {
		select {
		case <-gateway.release:
		case <-ctx.Done():
			return epusdt.CreateResponse{}, ctx.Err()
		}
	}
	if gateway.beforeReturn != nil {
		gateway.beforeReturn()
	}
	if gateway.failure != nil {
		return epusdt.CreateResponse{}, gateway.failure
	}
	return epusdt.CreateResponse{
		TradeID:        "trade-" + request.OrderID,
		OrderID:        request.OrderID,
		Amount:         request.Amount,
		Currency:       "usd",
		ActualAmount:   "1.23456789",
		ReceiveAddress: "T-integration-address",
		Token:          "usdt",
		Status:         1,
		ExpirationTime: time.Now().Add(20 * time.Minute).UTC().Truncate(time.Second),
		PaymentURL:     "https://pay.example/" + request.OrderID,
	}, nil
}

type gatewayIntegrationFixture struct {
	db       *sql.DB
	readerID int64
	service  func(Gateway) *Service
}

func newGatewayIntegrationFixture(t *testing.T) gatewayIntegrationFixture {
	t.Helper()
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	readerID := time.Now().UnixNano()
	username := "recharge-gateway-" + integrationtest.Prefix()
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
	return gatewayIntegrationFixture{
		db:       db,
		readerID: readerID,
		service: func(gateway Gateway) *Service {
			return NewService(SQLRepository{DB: db}, gateway, GatewaySnapshot{CredentialRef: "primary", MerchantPID: "merchant"}, nil)
		},
	}
}

type createOutcome struct {
	requestID string
	amount    string
	order     Order
	err       error
}

func TestConcurrentSameRequestCallsGatewayOnce(t *testing.T) {
	fixture := newGatewayIntegrationFixture(t)
	gateway := &integrationGateway{started: make(chan struct{}), release: make(chan struct{})}
	service := fixture.service(gateway)
	requestID := "same-" + integrationtest.Prefix()
	outcomes := make(chan createOutcome, 8)
	for range 8 {
		go func() {
			order, err := service.CreateOrder(context.Background(), CreateRequest{ReaderID: fixture.readerID, CustomDiamondAmount: "100", RequestID: requestID})
			outcomes <- createOutcome{requestID: requestID, amount: "100", order: order, err: err}
		}()
	}
	select {
	case <-gateway.started:
	case <-time.After(5 * time.Second):
		t.Fatal("gateway was not called")
	}
	var creating []createOutcome
	for range 7 {
		select {
		case outcome := <-outcomes:
			creating = append(creating, outcome)
		case <-time.After(5 * time.Second):
			t.Fatal("idempotent calls did not return while the gateway was blocked")
		}
	}
	if gateway.calls.Load() != 1 {
		t.Fatalf("gateway calls=%d", gateway.calls.Load())
	}
	var orderCount int
	if err := fixture.db.QueryRow(`SELECT count(*) FROM reader_recharge_orders WHERE reader_id=$1`, fixture.readerID).Scan(&orderCount); err != nil || orderCount != 1 {
		t.Fatalf("order count=%d err=%v", orderCount, err)
	}
	firstID := creating[0].order.ID
	for _, outcome := range creating {
		if outcome.err != nil || outcome.order.ID != firstID || outcome.order.Status != "creating" {
			t.Fatalf("creating outcome=%+v", outcome)
		}
	}
	close(gateway.release)
	var completed createOutcome
	select {
	case completed = <-outcomes:
	case <-time.After(5 * time.Second):
		t.Fatal("gateway owner did not complete")
	}
	if completed.err != nil || completed.order.ID != firstID || completed.order.Status != "pending" || gateway.calls.Load() != 1 {
		t.Fatalf("completed=%+v gateway calls=%d", completed, gateway.calls.Load())
	}
	assertGatewaySnapshot(t, fixture.db, completed.order)
}

func TestConcurrentDifferentRequestsDoNotCreateFalseIdempotency(t *testing.T) {
	fixture := newGatewayIntegrationFixture(t)
	gateway := &integrationGateway{started: make(chan struct{}), release: make(chan struct{})}
	service := fixture.service(gateway)
	requests := []CreateRequest{
		{ReaderID: fixture.readerID, CustomDiamondAmount: "100", RequestID: "amount-100-" + integrationtest.Prefix()},
		{ReaderID: fixture.readerID, CustomDiamondAmount: "200", RequestID: "amount-200-" + integrationtest.Prefix()},
	}
	outcomes := make(chan createOutcome, len(requests))
	for _, request := range requests {
		request := request
		go func() {
			order, err := service.CreateOrder(context.Background(), request)
			outcomes <- createOutcome{requestID: request.RequestID, amount: request.CustomDiamondAmount, order: order, err: err}
		}()
	}
	select {
	case <-gateway.started:
	case <-time.After(5 * time.Second):
		t.Fatal("gateway was not called")
	}
	var blocked createOutcome
	select {
	case blocked = <-outcomes:
	case <-time.After(5 * time.Second):
		t.Fatal("different request did not return the active creating order")
	}
	if blocked.err != nil || blocked.order.Status != "creating" || gateway.calls.Load() != 1 {
		t.Fatalf("blocked=%+v calls=%d", blocked, gateway.calls.Load())
	}
	close(gateway.release)
	var winner createOutcome
	select {
	case winner = <-outcomes:
	case <-time.After(5 * time.Second):
		t.Fatal("winning request did not complete")
	}
	if winner.err != nil || winner.order.Status != "pending" || winner.order.ID != blocked.order.ID {
		t.Fatalf("winner=%+v blocked=%+v", winner, blocked)
	}
	var storedRequestID, storedDiamonds string
	if err := fixture.db.QueryRow(`SELECT request_id,diamond_amount::text FROM reader_recharge_orders WHERE id=$1`, winner.order.ID).Scan(&storedRequestID, &storedDiamonds); err != nil {
		t.Fatal(err)
	}
	if storedRequestID != winner.requestID || storedDiamonds != winner.amount {
		t.Fatalf("stored request=%q diamonds=%q winner=%+v", storedRequestID, storedDiamonds, winner)
	}
	loser := requests[0]
	if loser.RequestID == winner.requestID {
		loser = requests[1]
	}
	retried, err := service.CreateOrder(context.Background(), loser)
	if err != nil || retried.ID == winner.order.ID || retried.DiamondAmount != loser.CustomDiamondAmount || retried.Status != "pending" || gateway.calls.Load() != 2 {
		t.Fatalf("retried=%+v err=%v calls=%d", retried, err, gateway.calls.Load())
	}
	var oldStatus string
	if err := fixture.db.QueryRow(`SELECT status FROM reader_recharge_orders WHERE id=$1`, winner.order.ID).Scan(&oldStatus); err != nil || oldStatus != "superseded" {
		t.Fatalf("old status=%q err=%v", oldStatus, err)
	}
	var orderCount, activeCount, requestCount int
	if err := fixture.db.QueryRow(`SELECT count(*),count(*) FILTER (WHERE active_reader_id IS NOT NULL),count(DISTINCT request_id) FROM reader_recharge_orders WHERE reader_id=$1`, fixture.readerID).Scan(&orderCount, &activeCount, &requestCount); err != nil {
		t.Fatal(err)
	}
	if orderCount != 2 || activeCount != 1 || requestCount != 2 {
		t.Fatalf("orders=%d active=%d requests=%d", orderCount, activeCount, requestCount)
	}
}

func TestGatewayUnknownBlocksDifferentRequestWithoutAnotherCall(t *testing.T) {
	fixture := newGatewayIntegrationFixture(t)
	gateway := &integrationGateway{failure: &epusdt.GatewayError{Code: "REQUEST_TIMEOUT", Class: epusdt.FailureUncertain, Summary: "timed out"}}
	service := fixture.service(gateway)
	first, err := service.CreateOrder(context.Background(), CreateRequest{ReaderID: fixture.readerID, CustomDiamondAmount: "100", RequestID: "unknown-" + integrationtest.Prefix()})
	if err != nil || first.Status != "gateway_unknown" || gateway.calls.Load() != 1 {
		t.Fatalf("first=%+v err=%v calls=%d", first, err, gateway.calls.Load())
	}
	second, err := service.CreateOrder(context.Background(), CreateRequest{ReaderID: fixture.readerID, CustomDiamondAmount: "200", RequestID: "blocked-" + integrationtest.Prefix()})
	if err != nil || second.ID != first.ID || second.Status != "gateway_unknown" || gateway.calls.Load() != 1 {
		t.Fatalf("second=%+v err=%v calls=%d", second, err, gateway.calls.Load())
	}
	var orderCount int
	if err := fixture.db.QueryRow(`SELECT count(*) FROM reader_recharge_orders WHERE reader_id=$1`, fixture.readerID).Scan(&orderCount); err != nil || orderCount != 1 {
		t.Fatalf("orders=%d err=%v", orderCount, err)
	}
}

func TestGatewaySuccessWithCanceledCompletionReturnsCorrelatableError(t *testing.T) {
	fixture := newGatewayIntegrationFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	gateway := &integrationGateway{beforeReturn: cancel}
	service := fixture.service(gateway)
	order, err := service.CreateOrder(ctx, CreateRequest{ReaderID: fixture.readerID, CustomDiamondAmount: "100", RequestID: "complete-failure-" + integrationtest.Prefix()})
	var persistenceErr *GatewayPersistenceError
	if !errors.Is(err, context.Canceled) || !errors.As(err, &persistenceErr) || persistenceErr.OrderID != order.ID || persistenceErr.OrderNo != order.OrderNo || order.ID == "" || gateway.calls.Load() != 1 {
		t.Fatalf("order=%+v err=%v persistence=%+v calls=%d", order, err, persistenceErr, gateway.calls.Load())
	}
	var status string
	var activeReaderID int64
	var gatewayTradeID sql.NullString
	if queryErr := fixture.db.QueryRow(`SELECT status,active_reader_id,gateway_trade_id FROM reader_recharge_orders WHERE id=$1`, order.ID).Scan(&status, &activeReaderID, &gatewayTradeID); queryErr != nil {
		t.Fatal(queryErr)
	}
	if status != "creating" || activeReaderID != fixture.readerID || gatewayTradeID.Valid {
		t.Fatalf("status=%s active=%d trade=%+v", status, activeReaderID, gatewayTradeID)
	}
}

func assertGatewaySnapshot(t *testing.T, db *sql.DB, order Order) {
	t.Helper()
	var tradeID, actualAmount, address, paymentURL, credentialRef, merchantPID string
	var gatewayStatus int
	var expires time.Time
	if err := db.QueryRow(`SELECT gateway_trade_id,actual_amount::text,receive_address,payment_url,gateway_status,expire_time,credential_ref,merchant_pid_snapshot FROM reader_recharge_orders WHERE id=$1`, order.ID).Scan(&tradeID, &actualAmount, &address, &paymentURL, &gatewayStatus, &expires, &credentialRef, &merchantPID); err != nil {
		t.Fatal(err)
	}
	if tradeID != "trade-"+order.OrderNo || actualAmount != "1.23456789" || address != "T-integration-address" || paymentURL != "https://pay.example/"+order.OrderNo || gatewayStatus != 1 || expires.IsZero() || credentialRef != "primary" || merchantPID != "merchant" {
		t.Fatalf("snapshot trade=%q actual=%q address=%q url=%q status=%d expires=%s credential=%q merchant=%q", tradeID, actualAmount, address, paymentURL, gatewayStatus, expires, credentialRef, merchantPID)
	}
	var activeCount, orderCount int
	if err := db.QueryRow(`SELECT count(*) FILTER (WHERE active_reader_id IS NOT NULL),count(*) FROM reader_recharge_orders WHERE reader_id=$1`, order.ReaderID).Scan(&activeCount, &orderCount); err != nil {
		t.Fatal(err)
	}
	if activeCount != 1 || orderCount != 1 {
		t.Fatalf("active=%d orders=%d", activeCount, orderCount)
	}
}
