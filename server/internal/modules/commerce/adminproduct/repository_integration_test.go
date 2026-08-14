//go:build integration

package adminproduct

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestProductCRUDAndReferencedDeleteProtection(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r := SQLRepository{DB: db}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	p, err := r.Create(ctx, Input{ProductType: "membership", ProductName: "集成会员-" + suffix, PriceCoin: "50", AllowBonusCoin: true, SaleStatus: "off_sale", SortOrder: 2})
	if err != nil || p.ID == "" || p.PriceCoin != "50" {
		t.Fatalf("created=%+v err=%v", p, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_purchase_orders WHERE product_id=$1`, p.ID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM commerce_products WHERE id=$1`, p.ID)
	})
	p, err = r.Update(ctx, parseProductID(p.ID), Input{ProductType: "membership", ProductName: p.ProductName, PriceCoin: "60", SaleStatus: "on_sale", SortOrder: 1})
	if err != nil || p.SaleStatus != "on_sale" || p.PriceCoin != "60" {
		t.Fatalf("updated=%+v err=%v", p, err)
	}
	items, total, err := r.List(ctx, p.ProductName, "membership", "on_sale", 1, 20)
	if err != nil || total != 1 || len(items) != 1 || items[0].ID != p.ID {
		t.Fatalf("items=%+v total=%d err=%v", items, total, err)
	}
	var readerID int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),2607300005000)+1 FROM reader_accounts`).Scan(&readerID); err != nil {
		t.Fatal(err)
	}
	username := "product-ref-" + suffix
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash) VALUES($1,$2,'fixture')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	var orderID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_purchase_orders(order_no,reader_id,order_type,product_id,product_type,product_name_snapshot,price_coin_snapshot,idempotency_key) VALUES($1,$2,'membership',$3,'membership',$4,60,$5) RETURNING id`, "PRODUCT-REF-"+suffix, readerID, p.ID, p.ProductName, "product-ref-"+suffix).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_purchase_orders WHERE id=$1`, orderID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})
	if err := r.Delete(ctx, parseProductID(p.ID)); err == nil {
		t.Fatal("expected referenced product delete conflict")
	}
}

func parseProductID(v string) int64 {
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}
