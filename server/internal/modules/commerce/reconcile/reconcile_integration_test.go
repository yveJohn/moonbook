//go:build integration

package reconcile

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestFullReconcileDetectsCrossDomainMismatch(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var readerID int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),9007199254743000)+100 FROM reader_accounts`).Scan(&readerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash) VALUES($1,$2,'fixture')`, readerID, fmt.Sprintf("reconcile-%d", readerID)); err != nil {
		t.Fatal(err)
	}
	var rechargeLedgerID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_wallet_ledgers(reader_id,ledger_no,biz_type,order_no,direction,coin_type,amount,balance_before,balance_after,idempotency_key) VALUES($1,$2,'recharge',$3,'income','recharge',25,0,25,$4) RETURNING id`, readerID, fmt.Sprintf("RECON-R-%d", readerID), fmt.Sprintf("RECON-R-ORDER-%d", readerID), fmt.Sprintf("recon-r-%d", readerID)).Scan(&rechargeLedgerID); err != nil {
		t.Fatal(err)
	}
	var rechargeOrderID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_recharge_orders(order_no,reader_id,request_id,source_type,diamond_amount,price_usdt,provider,currency,token,network,gateway_trade_id,actual_amount,receive_address,status,wallet_ledger_id,paid_time) VALUES($1,$2,$3,'custom',25,1.00,'epusdt','USDT','USDT','TRC20',$4,1.00,'T-FIXTURE','paid',$5,now()) RETURNING id`, fmt.Sprintf("RECON-R-ORDER-%d", readerID), readerID, fmt.Sprintf("recon-request-%d", readerID), fmt.Sprintf("recon-trade-%d", readerID), rechargeLedgerID).Scan(&rechargeOrderID); err != nil {
		t.Fatal(err)
	}
	var purchaseOrderID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_purchase_orders(order_no,reader_id,order_type,product_type,product_name_snapshot,price_coin_snapshot,recharge_coin_amount,bonus_coin_amount,status,idempotency_key,paid_time) VALUES($1,$2,'membership','membership','Fixture membership',25,25,0,'paid',$3,now()) RETURNING id`, fmt.Sprintf("RECON-P-ORDER-%d", readerID), readerID, fmt.Sprintf("recon-p-%d", readerID)).Scan(&purchaseOrderID); err != nil {
		t.Fatal(err)
	}
	var purchaseLedgerID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_wallet_ledgers(reader_id,ledger_no,biz_type,order_no,direction,coin_type,amount,balance_before,balance_after,idempotency_key) VALUES($1,$2,'purchase',$3,'expense','recharge',25,25,0,$4) RETURNING id`, readerID, fmt.Sprintf("RECON-P-%d", readerID), fmt.Sprintf("RECON-P-ORDER-%d", readerID), fmt.Sprintf("recon-p-ledger-%d", readerID)).Scan(&purchaseLedgerID); err != nil {
		t.Fatal(err)
	}
	var grantID int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),9007199254743000)+1 FROM commerce_membership_grants`).Scan(&grantID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_membership_grants(id,reader_id,grant_type,starts_at,permanent,status,source_type,source_ref) VALUES($1,$2,'purchase',now(),true,'active','order',$3)`, grantID, readerID, fmt.Sprintf("RECON-P-ORDER-%d", readerID)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		q := context.Background()
		_, _ = db.ExecContext(q, `DELETE FROM commerce_membership_grants WHERE id=$1`, grantID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_wallet_ledgers WHERE id IN ($1,$2)`, rechargeLedgerID, purchaseLedgerID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_purchase_orders WHERE id=$1`, purchaseOrderID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_recharge_orders WHERE id=$1`, rechargeOrderID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})
	r, err := Full(ctx, db)
	if err != nil || len(r.Mismatches) != 0 || r.RechargeOrders != 1 || r.PurchaseOrders != 1 || r.Callbacks != 0 {
		t.Fatalf("report=%+v err=%v", r, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE reader_recharge_orders SET wallet_ledger_id=NULL WHERE id=$1`, rechargeOrderID); err != nil {
		t.Fatal(err)
	}
	r, err = Full(ctx, db)
	if err != nil || len(r.Mismatches) == 0 {
		t.Fatalf("expected recharge mismatch report=%+v err=%v", r, err)
	}
}
