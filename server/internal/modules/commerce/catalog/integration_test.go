//go:build integration

package catalog

import (
	"context"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestCatalogAccessReadersPostgresEntitlementIsolation(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	reader1, reader2, book, chapter, bookProduct, chapterProduct, membership := base, base+1, base+2, base+3, base+4, base+5, base+6
	_, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'x','enabled'),($3,$4,'x','enabled')`, reader1, "catalog-it-1-"+integrationtest.Prefix(), reader2, "catalog-it-2-"+integrationtest.Prefix())
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO commerce_products(id,product_type,target_id,product_name,price_coin,sale_status) VALUES($1,'book',$2,'集成作品',99,'on_sale'),($3,'chapter',$4,'集成章节',7,'on_sale')`, bookProduct, book, chapterProduct, chapter)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO commerce_membership_grants(id,reader_id,grant_type,starts_at,permanent,status) VALUES($1,$2,'admin',now(),true,'active')`, membership, reader1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO commerce_entitlements(id,reader_id,entitlement_type,target_id,starts_at,permanent,status) VALUES($1,$2,'chapter',$3,now(),true,'active')`, membership+1, reader1, chapter)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		for _, q := range []string{`DELETE FROM commerce_entitlements WHERE reader_id IN ($1,$2)`, `DELETE FROM commerce_membership_grants WHERE reader_id IN ($1,$2)`, `DELETE FROM commerce_products WHERE id IN ($1,$2)`, `DELETE FROM reader_accounts WHERE id IN ($1,$2)`} {
			_, _ = db.ExecContext(cleanup, q, reader1, reader2)
		}
	})
	service := NewService(SQLRepository{DB: db})
	req := AccessRequest{BookID: book, ChapterID: chapter, ChargeMode: string(FixedPrice), ChapterWordCount: 7000}
	anonymous, err := service.AccessReader(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if anonymous.Readable || anonymous.AccessReason != string(LoginRequired) || anonymous.Purchasable {
		t.Fatalf("anonymous access=%+v", anonymous)
	}
	readerOne := req
	readerOne.ReaderID = &reader1
	owned, err := service.AccessReader(ctx, readerOne)
	if err != nil {
		t.Fatal(err)
	}
	if !owned.Readable || !owned.MembershipEntitled || !owned.ChapterPurchased || owned.AccessReason != string(ChapterOwned) {
		t.Fatalf("entitled access=%+v", owned)
	}
	readerTwo := req
	readerTwo.ReaderID = &reader2
	other, err := service.AccessReader(ctx, readerTwo)
	if err != nil {
		t.Fatal(err)
	}
	if other.Readable || other.MembershipEntitled || other.ChapterPurchased || other.AccessReason != string(BookPurchaseRequired) || !other.Purchasable {
		t.Fatalf("reader entitlement leaked=%+v", other)
	}
	batch, err := service.BatchAccessReader(ctx, []AccessRequest{readerOne, readerTwo})
	if err != nil || len(batch) != 2 || !batch[0].Readable || batch[1].Readable {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
}
