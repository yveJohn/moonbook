//go:build integration

package adminrechargeorder

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	readercontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/contract"
	readerprovider "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/provider"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

func TestAdminRechargeOrderListAndGet(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var readerID int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),2607300000000)+1 FROM reader_accounts`).Scan(&readerID); err != nil {
		t.Fatal(err)
	}
	username := "admin-order-fixture-" + strconv.FormatInt(readerID, 10)
	orderNo := "ADMIN-FIXTURE-" + strconv.FormatInt(readerID, 10)
	secondOrderNo := "ADMIN-SECOND-" + strconv.FormatInt(readerID, 10)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash) VALUES($1,$2,'fixture')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_reader_search_projection(reader_id,username,nickname,status) VALUES($1,$2,'','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	var orderID, secondOrderID int64
	err := db.QueryRowContext(ctx, `INSERT INTO reader_recharge_orders(order_no,reader_id,request_id,source_type,diamond_amount,price_usdt,provider,currency,token,network,status,created_at) VALUES($1,$2,'admin-fixture','custom',100,'1.00','epusdt','usd','usdt','tron','pending','2026-01-01 00:00:00+00') RETURNING id`, orderNo, readerID).Scan(&orderID)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_recharge_orders(order_no,reader_id,request_id,source_type,diamond_amount,price_usdt,provider,currency,token,network,status,created_at) VALUES($1,$2,'admin-fixture-second','custom',50,'0.50','epusdt','usd','usdt','tron','pending','2026-01-01 00:00:00+00') RETURNING id`, secondOrderNo, readerID).Scan(&secondOrderID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_recharge_orders WHERE id IN ($1,$2)`, orderID, secondOrderID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM commerce_reader_search_projection WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})
	var callbackID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_payment_callback_logs(provider,recharge_order_id,merchant_order_no,gateway_trade_id,payload_hash,signature_valid,processing_result,failure_reason,response_status,response_body,request_time) VALUES('epusdt',$1,$2,'GW-FIXTURE',repeat('a',64),true,'success','',200,'success',now()) RETURNING id`, orderID, orderNo).Scan(&callbackID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_payment_callback_logs WHERE id=$1`, callbackID)
	})
	r := SQLRepository{DB: db}
	rows, total, err := r.List(ctx, orderNo, "pending", 1, 20)
	if err != nil || total != 1 || len(rows) != 1 || rows[0].ID == "" {
		t.Fatalf("rows=%+v total=%d err=%v", rows, total, err)
	}
	firstPage, total, err := r.List(ctx, username, "pending", 1, 1)
	if err != nil || total != 2 || len(firstPage) != 1 || firstPage[0].ID != strconv.FormatInt(secondOrderID, 10) {
		t.Fatalf("first page=%+v total=%d err=%v", firstPage, total, err)
	}
	secondPage, total, err := r.List(ctx, username, "pending", 2, 1)
	if err != nil || total != 2 || len(secondPage) != 1 || secondPage[0].ID != strconv.FormatInt(orderID, 10) {
		t.Fatalf("second page=%+v total=%d err=%v", secondPage, total, err)
	}
	got, err := r.Get(ctx, orderID)
	if err != nil || got.OrderNo != orderNo || got.ReaderID == "" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	logs, logTotal, err := r.ListCallbacks(ctx, orderNo, "success", 1, 20)
	if err != nil || logTotal != 1 || len(logs) != 1 || logs[0].ID == "" || !logs[0].SignatureValid {
		t.Fatalf("logs=%+v total=%d err=%v", logs, logTotal, err)
	}
}

func TestManualPayIsIdempotentAndProtectsGatewayTradeID(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var readerID int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),2607300001000)+1 FROM reader_accounts`).Scan(&readerID); err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "manual-pay-fixture-" + suffix
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash) VALUES($1,$2,'fixture')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_reader_search_projection(reader_id,username,nickname,status) VALUES($1,$2,'','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	var orderID, disabledOrderID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_recharge_orders(order_no,reader_id,request_id,source_type,diamond_amount,price_usdt,provider,currency,token,network,status) VALUES($1,$2,$3,'custom',100,'1.00','epusdt','usd','usdt','tron','gateway_unknown') RETURNING id`, "MANUAL-PAY-FIXTURE-"+suffix, readerID, "manual-pay-fixture-"+suffix).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_recharge_orders(order_no,reader_id,request_id,source_type,diamond_amount,price_usdt,provider,currency,token,network,status) VALUES($1,$2,$3,'custom',50,'0.50','epusdt','usd','usdt','tron','gateway_unknown') RETURNING id`, "MANUAL-PAY-DISABLED-"+suffix, readerID, "manual-pay-disabled-"+suffix).Scan(&disabledOrderID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `UPDATE reader_accounts SET status='enabled' WHERE id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_payment_callback_logs WHERE recharge_order_id=$1`, orderID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_wallet_ledgers WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_wallets WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_recharge_orders WHERE id IN ($1,$2)`, orderID, disabledOrderID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})
	readerAccounts := readerprovider.NewAccount(db)
	service := NewService(SQLRepository{DB: db, Invites: readerprovider.NewInvite(db)}, transaction.New(db), readerAccounts)
	in := ManualPayInput{RequestID: "manual-request-1", GatewayTradeID: "manual-gateway-1", ActualAmount: "1.00", Remark: "核对后人工补单"}
	first, err := service.ManualPay(ctx, orderID, in)
	if err != nil || first.Status != "paid" || first.ID == "" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := service.ManualPay(ctx, orderID, in)
	if err != nil || second.Status != "paid" {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	var ledgerCount int
	var balance int64
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1 AND idempotency_key=$2`, readerID, "manual_recharge:"+first.ID+":manual-request-1").Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT recharge_coin_balance FROM reader_wallets WHERE reader_id=$1`, readerID).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 1 || balance != 100 {
		t.Fatalf("ledgerCount=%d balance=%d", ledgerCount, balance)
	}
	if _, err := service.ManualPay(ctx, orderID, ManualPayInput{RequestID: "manual-request-2", GatewayTradeID: "manual-gateway-2", ActualAmount: "1.00", Remark: "重复补单"}); err == nil {
		t.Fatal("expected paid order conflict")
	}
	if _, err := db.ExecContext(ctx, `UPDATE reader_accounts SET status='disabled' WHERE id=$1`, readerID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ManualPay(ctx, disabledOrderID, ManualPayInput{RequestID: "manual-disabled", GatewayTradeID: "manual-disabled", ActualAmount: "0.50", Remark: "禁用账号"}); !errors.Is(err, readercontract.ErrAccountDisabled) || apperror.Expose(err).Code != apperror.CodeConflict {
		t.Fatalf("disabled manual pay err=%v", err)
	}
	var disabledStatus string
	if err := db.QueryRowContext(ctx, `SELECT status FROM reader_recharge_orders WHERE id=$1`, disabledOrderID).Scan(&disabledStatus); err != nil || disabledStatus != "gateway_unknown" {
		t.Fatalf("disabled order status=%s err=%v", disabledStatus, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE reader_accounts SET status='deleted' WHERE id=$1`, readerID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ManualPay(ctx, disabledOrderID, ManualPayInput{RequestID: "manual-deleted", GatewayTradeID: "manual-deleted", ActualAmount: "0.50", Remark: "删除账号"}); !errors.Is(err, readercontract.ErrAccountNotFound) || apperror.Expose(err).Code != apperror.CodeNotFound {
		t.Fatalf("deleted reader manual pay err=%v", err)
	}
}
