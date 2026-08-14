//go:build integration

package recharge

import (
	"context"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestRechargeCatalogQuoteAndIdempotentOrder(t *testing.T) {
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
	if _, err := db.ExecContext(ctx, `UPDATE reader_payment_channels SET enabled=true WHERE provider='epusdt'`); err != nil {
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
	request := CreateRequest{ReaderID: readerID, ProductID: &productID, RequestID: "request-" + integrationtest.Prefix()}
	first, err := r.CreateOrder(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.CreateOrder(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || first.OrderNo != second.OrderNo || first.Status != "pending" {
		t.Fatalf("idempotency first=%+v second=%+v", first, second)
	}
	got, err := r.GetOrder(ctx, readerID, first.ID)
	if err != nil || got.ReaderID != first.ReaderID {
		t.Fatalf("order=%+v err=%v", got, err)
	}
}
