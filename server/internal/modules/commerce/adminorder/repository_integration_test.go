//go:build integration

package adminorder

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestListAndGetPurchaseOrders(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var readerID, orderID int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),2607300010000)+1 FROM reader_accounts`).Scan(&readerID); err != nil { t.Fatal(err) }
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash) VALUES($1,$2,'fixture')`, readerID, "admin-order-"+suffix); err != nil { t.Fatal(err) }
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_purchase_orders(order_no,reader_id,order_type,product_type,product_name_snapshot,price_coin_snapshot,recharge_coin_amount,bonus_coin_amount,status,idempotency_key,paid_time) VALUES($1,$2,'membership','membership','集成会员',60,40,20,'paid',$3,now()) RETURNING id`, "ADMIN-PURCHASE-"+suffix, readerID, "admin-purchase-"+suffix).Scan(&orderID); err != nil { t.Fatal(err) }
	t.Cleanup(func() { _, _ = db.ExecContext(context.Background(), `DELETE FROM reader_purchase_orders WHERE id=$1`, orderID); _, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID) })
	r := SQLRepository{DB: db}
	items, total, err := r.List(ctx, "ADMIN-PURCHASE-"+suffix, "membership", "paid", 1, 20)
	if err != nil || total != 1 || len(items) != 1 || items[0].ID == "" || items[0].ReaderID == "" || items[0].PriceCoin != "60" { t.Fatalf("items=%+v total=%d err=%v", items, total, err) }
	got, err := r.Get(ctx, orderID)
	if err != nil || got.ID == "" || got.ReaderUsername != "admin-order-"+suffix || got.BonusCoinAmount != "20" { t.Fatalf("got=%+v err=%v", got, err) }
}
