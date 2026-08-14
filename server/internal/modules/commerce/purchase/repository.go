package purchase

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/wallet"
)

var ErrInvalidRequest = errors.New("invalid purchase request")
var ErrQuoteChanged = errors.New("purchase quote changed")
var ErrProductUnavailable = errors.New("product unavailable")
var ErrInsufficientBalance = errors.New("wallet balance is insufficient")

type SQLRepository struct{ DB *sql.DB }

func validateRequest(id int64, request string) error {
	if id <= 0 || strings.TrimSpace(request) == "" || len(request) > 128 {
		return ErrInvalidRequest
	}
	return nil
}
func parseID(v string) (int64, error) {
	n, e := strconv.ParseInt(v, 10, 64)
	if e != nil || n <= 0 {
		return 0, ErrInvalidRequest
	}
	return n, nil
}
func lockReader(ctx context.Context, tx *sql.Tx, id int64) error {
	_, e := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, id)
	return e
}
func orderNo(prefix string, id int64) string {
	return fmt.Sprintf("%s%d%d", prefix, id, time.Now().UnixNano()%1000000)
}
func strPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func (r SQLRepository) BuyMembership(ctx context.Context, readerID int64, productID, requestID string) (Order, error) {
	if err := validateRequest(readerID, requestID); err != nil {
		return Order{}, err
	}
	pid, e := parseID(productID)
	if e != nil {
		return Order{}, e
	}
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return Order{}, e
	}
	defer tx.Rollback()
	if e = lockReader(ctx, tx, readerID); e != nil {
		return Order{}, e
	}
	if o, e := findOrderTx(ctx, tx, readerID, "membership:"+requestID); e == nil {
		return o, nil
	} else if e != sql.ErrNoRows {
		return Order{}, e
	}
	var productName string
	var price int64
	var allowBonus bool
	var duration sql.NullInt64
	var productType string
	err := tx.QueryRowContext(ctx, `SELECT product_name,price_coin,duration_days,product_type,allow_bonus_coin FROM commerce_products WHERE id=$1 AND product_type='membership' AND sale_status='on_sale'`, pid).Scan(&productName, &price, &duration, &productType, &allowBonus)
	if err != nil {
		return Order{}, ErrProductUnavailable
	}
	if price <= 0 {
		return Order{}, ErrProductUnavailable
	}
	o, err := insertOrderTx(ctx, tx, readerID, "membership", pid, productType, 0, "", productName, price, 0, 0, 0, "membership:"+requestID, orderNo("MBM", readerID))
	if err != nil {
		return Order{}, err
	}
	bonus, recharge, err := debitTx(ctx, tx, readerID, price, o.ID, o.OrderNo, "membership_purchase", allowBonus)
	if err != nil {
		return Order{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO commerce_membership_grants(reader_id,grant_type,starts_at,expires_at,permanent,status,source_type,source_ref) VALUES($1,'purchase',now(),CASE WHEN $2 THEN NULL ELSE now() + ($3::bigint::text || ' days')::interval END,$2,'active','order',$4)`, readerID, !duration.Valid, duration.Int64, o.OrderNo); err != nil {
		return Order{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE reader_purchase_orders SET recharge_coin_amount=$1,bonus_coin_amount=$2,paid_time=now(),updated_at=now() WHERE id=$3`, recharge, bonus, o.ID); err != nil {
		return Order{}, err
	}
	if err = tx.Commit(); err != nil {
		return Order{}, err
	}
	o.RechargeCoinAmount = strconv.FormatInt(recharge, 10)
	o.BonusCoinAmount = strconv.FormatInt(bonus, 10)
	o.Status = "paid"
	return o, nil
}

func (r SQLRepository) BuyChapter(ctx context.Context, readerID int64, chapterID, expectedPrice, requestID string) (ChapterResult, error) {
	if err := validateRequest(readerID, requestID); err != nil {
		return ChapterResult{}, err
	}
	cid, e := parseID(chapterID)
	if e != nil {
		return ChapterResult{}, e
	}
	expected, e := strconv.ParseInt(expectedPrice, 10, 64)
	if e != nil || expected < 0 {
		return ChapterResult{}, ErrInvalidRequest
	}
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return ChapterResult{}, e
	}
	defer tx.Rollback()
	if e = lockReader(ctx, tx, readerID); e != nil {
		return ChapterResult{}, e
	}
	key := "chapter:" + requestID
	if o, e := findOrderTx(ctx, tx, readerID, key); e == nil {
		return ChapterResult{PurchaseStatus: "paid", Order: &o}, nil
	} else if e != sql.ErrNoRows {
		return ChapterResult{}, e
	}
	var bookID, wordCount int64
	var chapterName string
	if e = tx.QueryRowContext(ctx, `SELECT book_id,word_count,chapter_name FROM novel_chapters WHERE id=$1 AND chapter_status='enabled' AND deleted_at IS NULL`, cid).Scan(&bookID, &wordCount, &chapterName); e != nil {
		return ChapterResult{}, ErrProductUnavailable
	}
	var wordUnit int
	var coinUnit int64
	var enabled bool
	if e = tx.QueryRowContext(ctx, `SELECT word_unit,coin_unit,enabled FROM commerce_chapter_pricing_config WHERE id=1`).Scan(&wordUnit, &coinUnit, &enabled); e != nil {
		return ChapterResult{}, e
	}
	price := int64(0)
	if enabled && wordCount > 0 {
		price = ((wordCount + int64(wordUnit) - 1) / int64(wordUnit)) * coinUnit
	}
	var productID sql.NullInt64
	var productName string
	var productPrice sql.NullInt64
	var allowBonus bool
	if e = tx.QueryRowContext(ctx, `SELECT id,product_name,price_coin,allow_bonus_coin FROM commerce_products WHERE product_type='chapter' AND target_id=$1 AND sale_status='on_sale'`, cid).Scan(&productID, &productName, &productPrice, &allowBonus); e == nil {
		price = productPrice.Int64
	} else if e != sql.ErrNoRows {
		return ChapterResult{}, e
	}
	q := ChapterQuote{ChapterID: chapterID, BookID: strconv.FormatInt(bookID, 10), WordCount: int(wordCount), WordUnit: wordUnit, CoinUnit: strconv.FormatInt(coinUnit, 10), PriceCoin: strconv.FormatInt(price, 10)}
	if price == 0 {
		return ChapterResult{PurchaseStatus: "free", Quote: q}, nil
	}
	if expected != price {
		return ChapterResult{PurchaseStatus: "quote_changed", Quote: q}, nil
	}
	var productArg int64
	if productID.Valid {
		productArg = productID.Int64
	}
	o, e := insertOrderTx(ctx, tx, readerID, "chapter", productArg, "chapter", cid, strconv.FormatInt(bookID, 10), chapterName, price, int(wordCount), wordUnit, coinUnit, key, orderNo("MBC", readerID))
	if e != nil {
		return ChapterResult{}, e
	}
	bonus, recharge, e := debitTx(ctx, tx, readerID, price, o.ID, o.OrderNo, "chapter_purchase", allowBonus)
	if e != nil {
		return ChapterResult{}, e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO commerce_entitlements(reader_id,entitlement_type,target_id,starts_at,permanent,status,source_type,source_ref) VALUES($1,'chapter',$2,now(),true,'active','order',$3) ON CONFLICT(reader_id,entitlement_type,target_id) DO NOTHING`, readerID, cid, o.OrderNo); e != nil {
		return ChapterResult{}, e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE reader_purchase_orders SET recharge_coin_amount=$1,bonus_coin_amount=$2,paid_time=now() WHERE id=$3`, recharge, bonus, o.ID); e != nil {
		return ChapterResult{}, e
	}
	if e = tx.Commit(); e != nil {
		return ChapterResult{}, e
	}
	o.RechargeCoinAmount = strconv.FormatInt(recharge, 10)
	o.BonusCoinAmount = strconv.FormatInt(bonus, 10)
	o.Status = "paid"
	return ChapterResult{PurchaseStatus: "paid", Quote: q, Order: &o}, nil
}

func (r SQLRepository) BuyBook(ctx context.Context, readerID int64, bookID, expectedPrice string) (Order, error) {
	if err := validateRequest(readerID, expectedPrice); err != nil {
		return Order{}, err
	}
	bid, e := parseID(bookID)
	if e != nil {
		return Order{}, e
	}
	expected, e := strconv.ParseInt(expectedPrice, 10, 64)
	if e != nil || expected < 0 {
		return Order{}, ErrInvalidRequest
	}
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return Order{}, e
	}
	defer tx.Rollback()
	if e = lockReader(ctx, tx, readerID); e != nil {
		return Order{}, e
	}
	key := fmt.Sprintf("book:%d:%s", bid, expectedPrice)
	if o, e := findOrderTx(ctx, tx, readerID, key); e == nil {
		return o, nil
	} else if e != sql.ErrNoRows {
		return Order{}, e
	}
	var pid, price int64
	var name string
	var allowBonus bool
	if e = tx.QueryRowContext(ctx, `SELECT id,product_name,price_coin,allow_bonus_coin FROM commerce_products WHERE product_type='book' AND target_id=$1 AND sale_status='on_sale'`, bid).Scan(&pid, &name, &price, &allowBonus); e != nil {
		return Order{}, ErrProductUnavailable
	}
	if expected != price {
		return Order{}, ErrQuoteChanged
	}
	o, e := insertOrderTx(ctx, tx, readerID, "book", pid, "book", bid, bookID, name, price, 0, 0, 0, key, orderNo("MBB", readerID))
	if e != nil {
		return Order{}, e
	}
	bonus, recharge, e := debitTx(ctx, tx, readerID, price, o.ID, o.OrderNo, "book_purchase", allowBonus)
	if e != nil {
		return Order{}, e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO commerce_entitlements(reader_id,entitlement_type,target_id,starts_at,permanent,status,source_type,source_ref) VALUES($1,'book',$2,now(),true,'active','order',$3) ON CONFLICT(reader_id,entitlement_type,target_id) DO NOTHING`, readerID, bid, o.OrderNo); e != nil {
		return Order{}, e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE reader_purchase_orders SET recharge_coin_amount=$1,bonus_coin_amount=$2,paid_time=now() WHERE id=$3`, recharge, bonus, o.ID); e != nil {
		return Order{}, e
	}
	if e = tx.Commit(); e != nil {
		return Order{}, e
	}
	o.RechargeCoinAmount = strconv.FormatInt(recharge, 10)
	o.BonusCoinAmount = strconv.FormatInt(bonus, 10)
	o.Status = "paid"
	return o, nil
}

func findOrderTx(ctx context.Context, tx *sql.Tx, readerID int64, key string) (Order, error) {
	var o Order
	err := tx.QueryRowContext(ctx, `SELECT id::text,reader_id::text,order_no,order_type,COALESCE(product_id::text,''),product_type,COALESCE(target_id::text,''),COALESCE(book_id_snapshot::text,''),product_name_snapshot,price_coin_snapshot::text,COALESCE(chapter_word_count_snapshot::text,''),COALESCE(pricing_word_unit_snapshot::text,''),COALESCE(pricing_coin_unit_snapshot::text,''),recharge_coin_amount::text,bonus_coin_amount::text,status,idempotency_key,remark,paid_time,created_at,updated_at FROM reader_purchase_orders WHERE reader_id=$1 AND idempotency_key=$2`, readerID, key).Scan(&o.ID, &o.ReaderID, &o.OrderNo, &o.OrderType, &o.ProductID, &o.ProductType, &o.TargetID, &o.BookIDSnapshot, &o.ProductName, &o.PriceCoin, &o.ChapterWordCount, &o.PricingWordUnit, &o.PricingCoinUnit, &o.RechargeCoinAmount, &o.BonusCoinAmount, &o.Status, &o.IdempotencyKey, &o.Remark, &o.PaidTime, &o.CreateTime, &o.UpdateTime)
	return o, err
}
func insertOrderTx(ctx context.Context, tx *sql.Tx, readerID int64, typ string, productID int64, productType string, targetID int64, bookID, name string, price int64, words, wordUnit int, coinUnit int64, key, order string) (Order, error) {
	var o Order
	o.ReaderID = strconv.FormatInt(readerID, 10)
	o.OrderNo = order
	o.OrderType = typ
	o.ProductID = strconv.FormatInt(productID, 10)
	o.ProductType = productType
	o.TargetID = strconv.FormatInt(targetID, 10)
	o.BookIDSnapshot = bookID
	o.ProductName = name
	o.PriceCoin = strconv.FormatInt(price, 10)
	o.ChapterWordCount = strconv.Itoa(words)
	o.PricingWordUnit = strconv.Itoa(wordUnit)
	o.PricingCoinUnit = strconv.FormatInt(coinUnit, 10)
	o.IdempotencyKey = key
	o.Status = "paid"
	err := tx.QueryRowContext(ctx, `INSERT INTO reader_purchase_orders(order_no,reader_id,order_type,product_id,product_type,target_id,book_id_snapshot,product_name_snapshot,price_coin_snapshot,chapter_word_count_snapshot,pricing_word_unit_snapshot,pricing_coin_unit_snapshot,idempotency_key,status,paid_time) VALUES($1,$2,$3,NULLIF($4::bigint,0),$5,NULLIF($6::bigint,0),NULLIF($7,'')::bigint,$8,$9,NULLIF($10::integer,0),NULLIF($11::integer,0),NULLIF($12::bigint,0),$13,'paid',now()) RETURNING id::text,paid_time,created_at,updated_at`, order, readerID, typ, productID, productType, targetID, bookID, name, price, words, wordUnit, coinUnit, key).Scan(&o.ID, &o.PaidTime, &o.CreateTime, &o.UpdateTime)
	return o, err
}
func debitTx(ctx context.Context, tx *sql.Tx, readerID, amount int64, orderID string, orderNo, biz string, allowBonus bool) (int64, int64, error) {
	var bonusBalance, rechargeBalance int64
	if e := tx.QueryRowContext(ctx, `INSERT INTO reader_wallets(reader_id) VALUES($1) ON CONFLICT(reader_id) DO NOTHING`, readerID).Err(); e != nil {
		return 0, 0, e
	}
	if e := tx.QueryRowContext(ctx, `SELECT bonus_coin_balance,recharge_coin_balance FROM reader_wallets WHERE reader_id=$1 FOR UPDATE`, readerID).Scan(&bonusBalance, &rechargeBalance); e != nil {
		return 0, 0, e
	}
	bonus := amount
	if !allowBonus {
		bonus = 0
	}
	if bonus > bonusBalance {
		bonus = bonusBalance
	}
	recharge := amount - bonus
	if recharge > rechargeBalance {
		return 0, 0, ErrInsufficientBalance
	}
	base := fmt.Sprintf("purchase:%s", orderID)
	if bonus > 0 {
		b := fmt.Sprintf("%s:bonus", base)
		if _, e := wallet.MutateTx(ctx, tx, wallet.Mutation{ReaderID: readerID, LedgerNo: "W" + orderID + "B", BizType: biz, BizID: strPtr(orderID), OrderNo: strPtr(orderNo), Direction: "expense", CoinType: "bonus", Amount: bonus, Remark: strPtr("购买消费"), IdempotencyKey: &b}); e != nil {
			return 0, 0, e
		}
	}
	if recharge > 0 {
		b := fmt.Sprintf("%s:recharge", base)
		if _, e := wallet.MutateTx(ctx, tx, wallet.Mutation{ReaderID: readerID, LedgerNo: "W" + orderID + "R", BizType: biz, BizID: strPtr(orderID), OrderNo: strPtr(orderNo), Direction: "expense", CoinType: "recharge", Amount: recharge, Remark: strPtr("购买消费"), IdempotencyKey: &b}); e != nil {
			return 0, 0, e
		}
	}
	return bonus, recharge, nil
}
