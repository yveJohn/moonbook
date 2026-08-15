//go:build integration

package purchase

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
	novelprovider "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/provider"
	readerprovider "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/provider"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

func TestMembershipAndBookPurchaseAreAtomicAndIdempotent(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	readerID, bookID, membershipID, bookProductID := base, base+1, base+2, base+3
	categoryID, authorID := base+4, base+5
	username := "purchase-it-" + integrationtest.Prefix()
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'x','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_reader_search_projection(reader_id,username,nickname,status) VALUES($1,$2,'','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_categories(id,category_code,category_name) VALUES($1,$2,$2)`, categoryID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_authors(id,pen_name,normalized_name,status,source) VALUES($1,$2,$2,'active','native')`, authorID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_books(id,primary_category_id,category_code,category_name,book_name,author_id,author_name,publish_status,source_type) VALUES($1,$2,$3,$3,'集成整书',$4,$3,'published','manual')`, bookID, categoryID, username, authorID); err != nil {
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
		_, _ = db.ExecContext(q, `DELETE FROM novel_books WHERE id=$1`, bookID)
		_, _ = db.ExecContext(q, `DELETE FROM novel_authors WHERE id=$1`, authorID)
		_, _ = db.ExecContext(q, `DELETE FROM novel_categories WHERE id=$1`, categoryID)
	})
	service := NewService(SQLRepository{DB: db}, transaction.New(db), readerprovider.NewAccount(db), novelprovider.NewPurchase(db))
	m, err := service.BuyMembership(ctx, readerID, fmtID(membershipID), "membership-"+integrationtest.Prefix())
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != "paid" || m.PaidTime == nil || m.CreateTime.IsZero() || m.UpdateTime.IsZero() {
		t.Fatalf("membership=%+v", m)
	}
	b, err := service.BuyBook(ctx, readerID, fmtID(bookID), "7")
	if err != nil {
		t.Fatal(err)
	}
	if b.BonusCoinAmount != "5" || b.RechargeCoinAmount != "2" {
		t.Fatalf("book debit=%+v", b)
	}
	repeat, err := service.BuyBook(ctx, readerID, fmtID(bookID), "7")
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

func TestConcurrentBookPurchaseDebitsWalletOnce(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	readerID, bookID, productID := base, base+1, base+2
	categoryID, authorID := base+3, base+4
	username := "purchase-concurrent-it-" + integrationtest.Prefix()
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'x','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_reader_search_projection(reader_id,username,nickname,status) VALUES($1,$2,'','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_categories(id,category_code,category_name) VALUES($1,$2,$2)`, categoryID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_authors(id,pen_name,normalized_name,status,source) VALUES($1,$2,$2,'active','native')`, authorID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_books(id,primary_category_id,category_code,category_name,book_name,author_id,author_name,publish_status,source_type) VALUES($1,$2,$3,$3,'并发整书',$4,$3,'published','manual')`, bookID, categoryID, username, authorID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_products(id,product_type,target_id,product_name,price_coin,allow_bonus_coin,sale_status) VALUES($1,'book',$2,'并发整书',7,true,'on_sale')`, productID, bookID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_wallets(reader_id,bonus_coin_balance,recharge_coin_balance) VALUES($1,5,5)`, readerID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		q := context.Background()
		_, _ = db.ExecContext(q, `DELETE FROM commerce_entitlements WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_purchase_orders WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_wallet_ledgers WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_wallets WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(q, `DELETE FROM commerce_products WHERE id=$1`, productID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_accounts WHERE id=$1`, readerID)
		_, _ = db.ExecContext(q, `DELETE FROM novel_books WHERE id=$1`, bookID)
		_, _ = db.ExecContext(q, `DELETE FROM novel_authors WHERE id=$1`, authorID)
		_, _ = db.ExecContext(q, `DELETE FROM novel_categories WHERE id=$1`, categoryID)
	})

	service := NewService(SQLRepository{DB: db}, transaction.New(db), readerprovider.NewAccount(db), novelprovider.NewPurchase(db))
	if _, err := service.BuyBook(ctx, readerID, fmtID(bookID), "6"); !errors.Is(err, ErrQuoteChanged) {
		t.Fatalf("stale quote err=%v", err)
	}

	const workers = 8
	start := make(chan struct{})
	results := make(chan Order, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			order, err := service.BuyBook(ctx, readerID, fmtID(bookID), "7")
			if err != nil {
				errs <- err
				return
			}
			results <- order
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent purchase: %v", err)
	}
	var orderID string
	for order := range results {
		if orderID == "" {
			orderID = order.ID
		}
		if order.ID != orderID || order.Status != "paid" {
			t.Fatalf("order=%+v firstID=%s", order, orderID)
		}
	}
	if orderID == "" {
		t.Fatal("no purchase result")
	}

	var bonusBalance, rechargeBalance, orderCount, ledgerCount, entitlementCount int
	if err := db.QueryRowContext(ctx, `SELECT bonus_coin_balance,recharge_coin_balance FROM reader_wallets WHERE reader_id=$1`, readerID).Scan(&bonusBalance, &rechargeBalance); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_purchase_orders WHERE reader_id=$1`, readerID).Scan(&orderCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1`, readerID).Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM commerce_entitlements WHERE reader_id=$1 AND entitlement_type='book' AND target_id=$2`, readerID, bookID).Scan(&entitlementCount); err != nil {
		t.Fatal(err)
	}
	if bonusBalance != 0 || rechargeBalance != 3 || orderCount != 1 || ledgerCount != 2 || entitlementCount != 1 {
		t.Fatalf("wallet=%d/%d orders=%d ledgers=%d entitlements=%d", bonusBalance, rechargeBalance, orderCount, ledgerCount, entitlementCount)
	}
}

func fmtID(v int64) string { return strconv.FormatInt(v, 10) }

type unavailableNovel struct{}

func (unavailableNovel) LockPurchaseSnapshot(context.Context, string, int64) (novelcontract.PurchaseSnapshot, error) {
	return novelcontract.PurchaseSnapshot{}, novelcontract.ErrUnavailable
}

func TestChapterQuotesAndDependencyFailuresAreAtomic(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	readerID, categoryID, authorID := base, base+1, base+2
	publishedBookID, draftBookID := base+3, base+4
	wordChapterID, freeChapterID, fixedChapterID, disabledChapterID := base+5, base+6, base+7, base+8
	fixedProductID, draftBookProductID := base+9, base+10
	username := "purchase-chapter-it-" + integrationtest.Prefix()
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'x','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_reader_search_projection(reader_id,username,nickname,status) VALUES($1,$2,'','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_categories(id,category_code,category_name) VALUES($1,$2,$2)`, categoryID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_authors(id,pen_name,normalized_name,status,source) VALUES($1,$2,$2,'active','native')`, authorID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_books(id,primary_category_id,category_code,category_name,book_name,author_id,author_name,publish_status,source_type) VALUES($1,$2,$3,$3,'计价作品',$4,$3,'published','manual'),($5,$2,$3,$3,'草稿作品',$4,$3,'draft','manual')`, publishedBookID, categoryID, username, authorID, draftBookID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_chapters(id,book_id,chapter_no,chapter_name,word_count,chapter_status,ai_clean_status,source_type) VALUES($1,$2,1,'字数章节',1201,'enabled','pending','manual'),($3,$2,2,'免费章节',0,'enabled','pending','manual'),($4,$2,3,'固定价章节',3000,'enabled','pending','manual'),($5,$2,4,'禁用章节',500,'disabled','pending','manual')`, wordChapterID, publishedBookID, freeChapterID, fixedChapterID, disabledChapterID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_products(id,product_type,target_id,product_name,price_coin,allow_bonus_coin,sale_status) VALUES($1,'chapter',$2,'固定价章节',9,false,'on_sale'),($3,'book',$4,'草稿整书',5,false,'on_sale')`, fixedProductID, fixedChapterID, draftBookProductID, draftBookID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_wallets(reader_id,recharge_coin_balance) VALUES($1,100)`, readerID); err != nil {
		t.Fatal(err)
	}
	var oldWordUnit int
	var oldCoinUnit int64
	var oldEnabled bool
	if err := db.QueryRowContext(ctx, `SELECT word_unit,coin_unit,enabled FROM commerce_chapter_pricing_config WHERE id=1`).Scan(&oldWordUnit, &oldCoinUnit, &oldEnabled); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE commerce_chapter_pricing_config SET word_unit=1000,coin_unit=2,enabled=true WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		q := context.Background()
		_, _ = db.ExecContext(q, `UPDATE reader_accounts SET status='enabled' WHERE id=$1`, readerID)
		_, _ = db.ExecContext(q, `UPDATE commerce_chapter_pricing_config SET word_unit=$1,coin_unit=$2,enabled=$3 WHERE id=1`, oldWordUnit, oldCoinUnit, oldEnabled)
		_, _ = db.ExecContext(q, `DELETE FROM commerce_entitlements WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_purchase_orders WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_wallet_ledgers WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_wallets WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(q, `DELETE FROM commerce_products WHERE id IN ($1,$2)`, fixedProductID, draftBookProductID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_accounts WHERE id=$1`, readerID)
		_, _ = db.ExecContext(q, `DELETE FROM novel_chapters WHERE id IN ($1,$2,$3,$4)`, wordChapterID, freeChapterID, fixedChapterID, disabledChapterID)
		_, _ = db.ExecContext(q, `DELETE FROM novel_books WHERE id IN ($1,$2)`, publishedBookID, draftBookID)
		_, _ = db.ExecContext(q, `DELETE FROM novel_authors WHERE id=$1`, authorID)
		_, _ = db.ExecContext(q, `DELETE FROM novel_categories WHERE id=$1`, categoryID)
	})

	service := NewService(SQLRepository{DB: db}, transaction.New(db), readerprovider.NewAccount(db), novelprovider.NewPurchase(db))
	word, err := service.BuyChapter(ctx, readerID, fmtID(wordChapterID), "4", "word-price")
	if err != nil || word.PurchaseStatus != "paid" || word.Order == nil || word.Quote.WordCount != 1201 || word.Quote.PriceCoin != "4" {
		t.Fatalf("word result=%+v err=%v", word, err)
	}
	free, err := service.BuyChapter(ctx, readerID, fmtID(freeChapterID), "0", "free")
	if err != nil || free.PurchaseStatus != "free" || free.Order != nil || free.Quote.PriceCoin != "0" {
		t.Fatalf("free result=%+v err=%v", free, err)
	}
	changed, err := service.BuyChapter(ctx, readerID, fmtID(fixedChapterID), "8", "fixed-stale")
	if err != nil || changed.PurchaseStatus != "quote_changed" || changed.Order != nil || changed.Quote.PriceCoin != "9" {
		t.Fatalf("changed result=%+v err=%v", changed, err)
	}
	fixed, err := service.BuyChapter(ctx, readerID, fmtID(fixedChapterID), "9", "fixed-paid")
	if err != nil || fixed.PurchaseStatus != "paid" || fixed.Order == nil || fixed.Order.ProductID != fmtID(fixedProductID) {
		t.Fatalf("fixed result=%+v err=%v", fixed, err)
	}
	if _, err := service.BuyChapter(ctx, readerID, fmtID(disabledChapterID), "2", "disabled"); !errors.Is(err, ErrProductUnavailable) {
		t.Fatalf("disabled chapter err=%v", err)
	}
	if _, err := service.BuyBook(ctx, readerID, fmtID(draftBookID), "5"); !errors.Is(err, ErrProductUnavailable) {
		t.Fatalf("draft book err=%v", err)
	}
	failingService := NewService(SQLRepository{DB: db}, transaction.New(db), readerprovider.NewAccount(db), unavailableNovel{})
	if _, err := failingService.BuyChapter(ctx, readerID, fmtID(wordChapterID), "4", "provider-failure"); !errors.Is(err, novelcontract.ErrUnavailable) {
		t.Fatalf("provider failure err=%v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE reader_accounts SET status='disabled' WHERE id=$1`, readerID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.BuyChapter(ctx, readerID, fmtID(wordChapterID), "4", "disabled-reader"); !errors.Is(err, ErrProductUnavailable) {
		t.Fatalf("disabled reader err=%v", err)
	}

	var orderCount, ledgerCount, entitlementCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_purchase_orders WHERE reader_id=$1`, readerID).Scan(&orderCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1`, readerID).Scan(&ledgerCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM commerce_entitlements WHERE reader_id=$1`, readerID).Scan(&entitlementCount); err != nil {
		t.Fatal(err)
	}
	if orderCount != 2 || ledgerCount != 2 || entitlementCount != 2 {
		t.Fatalf("orders=%d ledgers=%d entitlements=%d", orderCount, ledgerCount, entitlementCount)
	}
}
