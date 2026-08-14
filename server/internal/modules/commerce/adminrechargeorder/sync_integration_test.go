//go:build integration

package adminrechargeorder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/payment"
)

func TestPaymentSyncSettlesVerifiedGatewayResponseAndIsIdempotent(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	readerID, orderID := base, base+1
	orderNo := "SYNC-" + integrationtest.Prefix()
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'fixture','enabled')`, readerID, "sync-"+integrationtest.Prefix()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_recharge_orders(id,order_no,reader_id,request_id,source_type,diamond_amount,price_usdt,provider,currency,token,network,status) VALUES($1,$2,$3,$4,'custom',19,'2.00','epusdt','usd','usdt','tron','pending')`, orderID, orderNo, readerID, "request-"+integrationtest.Prefix()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_payment_callback_logs WHERE recharge_order_id=$1`, orderID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_wallet_ledgers WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_wallets WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_recharge_orders WHERE id=$1`, orderID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})
	secret := "sync-secret"
	t.Setenv("MOONBOOK_EPUSDT_PID", "sync-pid")
	t.Setenv("MOONBOOK_EPUSDT_SECRET", secret)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]string
		if json.NewDecoder(r.Body).Decode(&request) != nil || request["order_id"] != orderNo {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		fields := map[string]string{"pid": "sync-pid", "trade_id": "trade-1", "order_id": orderNo, "amount": "2.00", "actual_amount": "2.00", "receive_address": "T-test", "token": "usdt", "block_transaction_id": "tx-1", "status": "2"}
		fields["signature"] = payment.Sign(fields, secret)
		_ = json.NewEncoder(w).Encode(fields)
	}))
	defer server.Close()
	t.Setenv("MOONBOOK_EPUSDT_SYNC_URL", server.URL)
	r := SQLRepository{DB: db}
	o, err := r.Sync(ctx, orderID)
	if err != nil || o.Status != "paid" || o.GatewayTradeID != "trade-1" {
		t.Fatalf("order=%+v err=%v", o, err)
	}
	var balance int64
	if err := db.QueryRowContext(ctx, `SELECT recharge_coin_balance FROM reader_wallets WHERE reader_id=$1`, readerID).Scan(&balance); err != nil || balance != 19 {
		t.Fatalf("balance=%d err=%v", balance, err)
	}
	second, err := r.Sync(ctx, orderID)
	if err != nil || second.Status != "paid" {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	var ledgerCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1`, readerID).Scan(&ledgerCount); err != nil || ledgerCount != 1 {
		t.Fatalf("ledgerCount=%d err=%v", ledgerCount, err)
	}
}
