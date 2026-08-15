//go:build integration

package adminproduct

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/provider"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
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

func TestProductTargetsUseNovelProviderAndRejectUnavailableContent(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	categoryID, authorID := base, base+1
	publishedBookID, draftBookID := base+2, base+3
	enabledChapterID, disabledChapterID := base+4, base+5
	code := fmt.Sprintf("admin-product-target-%d", base)
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_categories(id,category_code,category_name) VALUES($1,$2,$2)`, categoryID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_authors(id,pen_name,normalized_name,status,source) VALUES($1,$2,$2,'active','native')`, authorID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_books(id,primary_category_id,category_code,category_name,book_name,author_id,author_name,publish_status,source_type) VALUES($1,$2,$3,$3,'可售作品',$4,$3,'published','manual'),($5,$2,$3,$3,'草稿作品',$4,$3,'draft','manual')`, publishedBookID, categoryID, code, authorID, draftBookID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_chapters(id,book_id,chapter_no,chapter_name,chapter_status,ai_clean_status,source_type) VALUES($1,$2,1,'可售章节','enabled','pending','manual'),($3,$2,2,'禁用章节','disabled','pending','manual')`, enabledChapterID, publishedBookID, disabledChapterID); err != nil {
		t.Fatal(err)
	}
	service := NewService(SQLRepository{DB: db}, provider.NewPurchase(db))
	bookProduct, err := service.Create(ctx, Input{ProductType: "book", TargetID: strconv.FormatInt(publishedBookID, 10), ProductName: "可售作品", PriceCoin: "10", SaleStatus: "on_sale"})
	if err != nil {
		t.Fatal(err)
	}
	chapterProduct, err := service.Create(ctx, Input{ProductType: "chapter", TargetID: strconv.FormatInt(enabledChapterID, 10), ProductName: "可售章节", PriceCoin: "2", SaleStatus: "on_sale"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		q := context.Background()
		_, _ = db.ExecContext(q, `DELETE FROM commerce_products WHERE id IN ($1,$2)`, bookProduct.ID, chapterProduct.ID)
		_, _ = db.ExecContext(q, `DELETE FROM novel_chapters WHERE id IN ($1,$2)`, enabledChapterID, disabledChapterID)
		_, _ = db.ExecContext(q, `DELETE FROM novel_books WHERE id IN ($1,$2)`, publishedBookID, draftBookID)
		_, _ = db.ExecContext(q, `DELETE FROM novel_authors WHERE id=$1`, authorID)
		_, _ = db.ExecContext(q, `DELETE FROM novel_categories WHERE id=$1`, categoryID)
	})
	for name, input := range map[string]Input{
		"draft book":       {ProductType: "book", TargetID: strconv.FormatInt(draftBookID, 10), ProductName: "草稿作品", PriceCoin: "1", SaleStatus: "on_sale"},
		"disabled chapter": {ProductType: "chapter", TargetID: strconv.FormatInt(disabledChapterID, 10), ProductName: "禁用章节", PriceCoin: "1", SaleStatus: "on_sale"},
		"missing chapter":  {ProductType: "chapter", TargetID: strconv.FormatInt(base+999, 10), ProductName: "缺失章节", PriceCoin: "1", SaleStatus: "on_sale"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := service.Create(ctx, input); err == nil || apperror.Expose(err).Code != apperror.CodeInvalidArgument {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
