//go:build integration

package adminrechargeorder

import (
	"context"
	"testing"
	"time"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestAdminRechargeOrderListAndGet(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var readerID int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),2607300000000)+1 FROM reader_accounts`).Scan(&readerID); err != nil { t.Fatal(err) }
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash) VALUES($1,$2,'fixture')`, readerID, "admin-order-fixture"); err != nil { t.Fatal(err) }
	var orderID int64
	err := db.QueryRowContext(ctx, `INSERT INTO reader_recharge_orders(order_no,reader_id,request_id,source_type,diamond_amount,price_usdt,provider,currency,token,network,status) VALUES('ADMIN-FIXTURE',$1,'admin-fixture','custom',100,'1.00','epusdt','usd','usdt','tron','pending') RETURNING id`, readerID).Scan(&orderID)
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _, _ = db.ExecContext(context.Background(), `DELETE FROM reader_recharge_orders WHERE id=$1`, orderID); _, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID) })
	r := SQLRepository{DB: db}
	rows, total, err := r.List(ctx, "ADMIN-FIXTURE", "pending", 1, 20)
	if err != nil || total != 1 || len(rows) != 1 || rows[0].ID == "" { t.Fatalf("rows=%+v total=%d err=%v", rows, total, err) }
	got, err := r.Get(ctx, orderID)
	if err != nil || got.OrderNo != "ADMIN-FIXTURE" || got.ReaderID == "" { t.Fatalf("got=%+v err=%v", got, err) }
}
