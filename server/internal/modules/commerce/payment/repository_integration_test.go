//go:build integration

package payment

import (
	"context"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestCallbackCreditsRechargeExactlyOnce(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	readerID, orderID := base, base+1
	username := "payment-it-" + integrationtest.Prefix()
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'x','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_wallets(reader_id) VALUES($1)`, readerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE reader_payment_channels SET enabled=true WHERE provider='epusdt'`); err != nil {
		t.Fatal(err)
	}
	orderNo := "PAY" + integrationtest.Prefix()
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_recharge_orders(id,order_no,reader_id,request_id,source_type,diamond_amount,price_usdt,provider,currency,token,network,status) VALUES($1,$2,$3,$4,'custom',17,2.00,'epusdt','usd','usdt','tron','pending')`, orderID, orderNo, readerID, "req-"+integrationtest.Prefix()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		q := context.Background()
		_, _ = db.ExecContext(q, `DELETE FROM reader_payment_callback_logs WHERE recharge_order_id=$1`, orderID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_wallet_ledgers WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_recharge_orders WHERE id=$1`, orderID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_wallets WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(q, `UPDATE reader_payment_channels SET enabled=false WHERE provider='epusdt'`)
		_, _ = db.ExecContext(q, `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})
	fields := map[string]string{"pid": "merchant", "trade_id": "trade-1", "order_id": orderNo, "amount": "2.00", "actual_amount": "2.00", "receive_address": "Taddress", "token": "USDT", "block_transaction_id": "tx-1", "status": "2"}
	fields["signature"] = Sign(fields, "secret")
	callback := Callback{PID: fields["pid"], TradeID: fields["trade_id"], OrderNo: orderNo, Amount: fields["amount"], ActualAmount: fields["actual_amount"], ReceiveAddress: fields["receive_address"], Token: fields["token"], TransactionID: fields["block_transaction_id"], Status: 2, Fields: fields}
	r := SQLRepository{DB: db}
	if err := r.Process(ctx, callback); err != nil {
		t.Fatal(err)
	}
	if err := r.Process(ctx, callback); err != nil {
		t.Fatal(err)
	}
	var balance, ledgers int64
	if err := db.QueryRowContext(ctx, `SELECT recharge_coin_balance FROM reader_wallets WHERE reader_id=$1`, readerID).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1`, readerID).Scan(&ledgers); err != nil {
		t.Fatal(err)
	}
	if balance != 17 || ledgers != 1 {
		t.Fatalf("balance=%d ledgers=%d", balance, ledgers)
	}
}
