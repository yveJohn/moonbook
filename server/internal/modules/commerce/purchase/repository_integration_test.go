//go:build integration

package purchase

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestMembershipAndBookPurchaseAreAtomicAndIdempotent(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	readerID, bookID, membershipID, bookProductID := base, base+1, base+2, base+3
	username := "purchase-it-" + integrationtest.Prefix()
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'x','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_products(id,product_type,target_id,product_name,price_coin,allow_bonus_coin,duration_days,sale_status) VALUES($1,'membership',NULL,'集成会员',3,false,30,'on_sale'),($2,'book',$3,'集成整书',7,true,NULL,'on_sale')`, membershipID, bookProductID, bookID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_wallets(reader_id,bonus_coin_balance,recharge_coin_balance) VALUES($1,5,5)`, readerID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		q := context.Background()
		_, _ = db.ExecContext(q, `DELETE FROM commerce_entitlements WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(q, `DELETE FROM commerce_membership_grants WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_purchase_orders WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_wallet_ledgers WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_wallets WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(q, `DELETE FROM commerce_products WHERE id IN ($1,$2)`, membershipID, bookProductID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})
	r := SQLRepository{DB: db}
	m, err := r.BuyMembership(ctx, readerID, fmtID(membershipID), "membership-"+integrationtest.Prefix())
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != "paid" || m.PaidTime == nil || m.CreateTime.IsZero() || m.UpdateTime.IsZero() {
		t.Fatalf("membership=%+v", m)
	}
	b, err := r.BuyBook(ctx, readerID, fmtID(bookID), "7")
	if err != nil {
		t.Fatal(err)
	}
	if b.BonusCoinAmount != "5" || b.RechargeCoinAmount != "2" {
		t.Fatalf("book debit=%+v", b)
	}
	repeat, err := r.BuyBook(ctx, readerID, fmtID(bookID), "7")
	if err != nil || repeat.ID != b.ID || repeat.PaidTime == nil || repeat.CreateTime.IsZero() || repeat.UpdateTime.IsZero() {
		t.Fatalf("repeat=%+v err=%v", repeat, err)
	}
	var balance int64
	if err = db.QueryRowContext(ctx, `SELECT recharge_coin_balance+bonus_coin_balance FROM reader_wallets WHERE reader_id=$1`, readerID).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if balance != 0 {
		t.Fatalf("balance=%d", balance)
	}
}
func fmtID(v int64) string { return strconv.FormatInt(v, 10) }
