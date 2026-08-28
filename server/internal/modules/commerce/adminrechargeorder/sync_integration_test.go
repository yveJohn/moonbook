//go:build integration

package adminrechargeorder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/adminpayment"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/epusdt"
	readerprovider "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/provider"
)

func TestPaymentSyncMarksUnsignedPaidStatusWithoutCrediting(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	readerID, orderID := base, base+1
	orderNo := "SYNC-" + integrationtest.Prefix()
	username := "sync-" + integrationtest.Prefix()
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'fixture','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_reader_search_projection(reader_id,username,nickname,status) VALUES($1,$2,'','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_recharge_orders(id,order_no,reader_id,request_id,source_type,diamond_amount,price_usdt,provider,currency,token,network,gateway_trade_id,status,active_reader_id) VALUES($1,$2,$3,$4,'custom',19,'2.00','epusdt','usd','usdt','tron','trade-1','pending',$3)`, orderID, orderNo, readerID, "request-"+integrationtest.Prefix()); err != nil {
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/pay/check-status/trade-1" {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status_code": 200, "message": "success", "data": map[string]any{"trade_id": "trade-1", "status": 2}})
	}))
	defer server.Close()
	credentials, err := epusdt.NewCredentialProvider("channel-test", "sync-pid", "sync-secret", "[]")
	if err != nil {
		t.Fatal(err)
	}
	endpoint, _ := url.Parse(server.URL)
	runtimeConfig := adminpayment.RuntimeConfig{SyncURL: server.URL + "/pay/check-status/{trade_id}", EPUSDT: epusdt.Config{Enabled: true, Credentials: credentials, CreateURL: endpoint, NotifyURL: endpoint, RedirectURL: endpoint, ConnectTimeout: time.Second, RequestTimeout: 5 * time.Second}}
	r := SQLRepository{DB: db, Invites: readerprovider.NewInvite(db), Accounts: readerprovider.NewAccount(db), PaymentRuntime: func(context.Context) (adminpayment.RuntimeConfig, error) { return runtimeConfig, nil }}
	o, err := r.Sync(ctx, orderID)
	if err != nil || o.Status != "callback_exception" || o.GatewayTradeID != "trade-1" || o.GatewayStatus == nil || *o.GatewayStatus != 2 {
		t.Fatalf("order=%+v err=%v", o, err)
	}
	var ledgerCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1`, readerID).Scan(&ledgerCount); err != nil || ledgerCount != 0 {
		t.Fatalf("ledgerCount=%d err=%v", ledgerCount, err)
	}
	second, err := r.Sync(ctx, orderID)
	if err != nil || second.Status != "callback_exception" {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	var auditCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_payment_callback_logs WHERE recharge_order_id=$1 AND processing_result='paid_no_callback'`, orderID).Scan(&auditCount); err != nil || auditCount != 2 {
		t.Fatalf("auditCount=%d err=%v", auditCount, err)
	}
}
