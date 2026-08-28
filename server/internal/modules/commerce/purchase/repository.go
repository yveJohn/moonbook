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
	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

var ErrInvalidRequest = errors.New("invalid purchase request")
var ErrQuoteChanged = errors.New("purchase quote changed")
var ErrProductUnavailable = errors.New("product unavailable")
var ErrInsufficientBalance = errors.New("wallet balance is insufficient")
var ErrUnavailable = errors.New("purchase dependency unavailable")

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
func orderNo(prefix string, id int64) string {
	return fmt.Sprintf("%s%d%d", prefix, id, time.Now().UnixNano()%1000000)
}
func strPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func (r SQLRepository) executor(ctx context.Context) (transaction.DBTX, error) {
	executor := transaction.Executor(ctx, r.DB)
	if executor == r.DB {
		return nil, transaction.ErrNoTransaction
	}
	return executor, nil
}

func (r SQLRepository) LockIdempotency(ctx context.Context, readerID int64) error {
	executor, err := r.executor(ctx)
	if err != nil {
		return err
	}
	_, err = executor.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, readerID)
	return err
}

func (r SQLRepository) FindOrder(ctx context.Context, readerID int64, key string) (Order, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Order{}, err
	}
	return findOrderTx(ctx, executor, readerID, key)
}

func (r SQLRepository) BuyMembership(ctx context.Context, readerID, productID int64, key string) (Order, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Order{}, err
	}
	var productName string
	var price int64
	var duration sql.NullInt64
	var productType string
	err = executor.QueryRowContext(ctx, `SELECT product_name,price_coin,duration_days,product_type FROM commerce_products WHERE id=$1 AND product_type='membership' AND sale_status='on_sale'`, productID).Scan(&productName, &price, &duration, &productType)
	if err != nil {
		return Order{}, ErrProductUnavailable
	}
	if price <= 0 {
		return Order{}, ErrProductUnavailable
	}
	o, err := insertOrderTx(ctx, executor, readerID, "membership", productID, productType, 0, "", productName, price, 0, 0, 0, key, orderNo("MBM", readerID))
	if err != nil {
		return Order{}, err
	}
	bonus, recharge, err := debitRechargeOnlyTx(ctx, executor, readerID, price, o.ID, o.OrderNo, "membership_purchase")
	if err != nil {
		return Order{}, err
	}
	if _, err = executor.ExecContext(ctx, `INSERT INTO commerce_membership_grants(reader_id,grant_type,starts_at,expires_at,permanent,status,source_type,source_ref) VALUES($1,'purchase',now(),CASE WHEN $2 THEN NULL ELSE now() + ($3::bigint::text || ' days')::interval END,$2,'active','order',$4)`, readerID, !duration.Valid, duration.Int64, o.OrderNo); err != nil {
		return Order{}, err
	}
	if _, err = executor.ExecContext(ctx, `UPDATE reader_purchase_orders SET recharge_coin_amount=$1,bonus_coin_amount=$2,paid_time=now(),updated_at=now() WHERE id=$3`, recharge, bonus, o.ID); err != nil {
		return Order{}, err
	}
	o.RechargeCoinAmount = strconv.FormatInt(recharge, 10)
	o.BonusCoinAmount = strconv.FormatInt(bonus, 10)
	o.Status = "paid"
	return o, nil
}

func (r SQLRepository) BuyChapter(ctx context.Context, readerID int64, snapshot novelcontract.PurchaseSnapshot, expected int64, key string) (ChapterResult, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return ChapterResult{}, err
	}
	var wordUnit int
	var coinUnit int64
	var enabled bool
	if err = executor.QueryRowContext(ctx, `SELECT word_unit,coin_unit,enabled FROM commerce_chapter_pricing_config WHERE id=1`).Scan(&wordUnit, &coinUnit, &enabled); err != nil {
		return ChapterResult{}, err
	}
	price := int64(0)
	if enabled && snapshot.WordCount > 0 {
		price = ((int64(snapshot.WordCount) + int64(wordUnit) - 1) / int64(wordUnit)) * coinUnit
	}
	var productID sql.NullInt64
	var productName string
	var productPrice sql.NullInt64
	if err = executor.QueryRowContext(ctx, `SELECT id,product_name,price_coin FROM commerce_products WHERE product_type='chapter' AND target_id=$1 AND sale_status='on_sale'`, snapshot.TargetID).Scan(&productID, &productName, &productPrice); err == nil {
		price = productPrice.Int64
	} else if err != sql.ErrNoRows {
		return ChapterResult{}, err
	}
	q := ChapterQuote{ChapterID: strconv.FormatInt(snapshot.TargetID, 10), BookID: strconv.FormatInt(snapshot.BookID, 10), WordCount: snapshot.WordCount, WordUnit: wordUnit, CoinUnit: strconv.FormatInt(coinUnit, 10), PriceCoin: strconv.FormatInt(price, 10)}
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
	name := snapshot.Name
	if productID.Valid {
		name = productName
	}
	o, err := insertOrderTx(ctx, executor, readerID, "chapter", productArg, "chapter", snapshot.TargetID, strconv.FormatInt(snapshot.BookID, 10), name, price, snapshot.WordCount, wordUnit, coinUnit, key, orderNo("MBC", readerID))
	if err != nil {
		return ChapterResult{}, err
	}
	bonus, recharge, err := debitMixedTx(ctx, executor, readerID, price, o.ID, o.OrderNo, "chapter_purchase")
	if err != nil {
		return ChapterResult{}, err
	}
	if _, err = executor.ExecContext(ctx, `INSERT INTO commerce_entitlements(reader_id,entitlement_type,target_id,starts_at,permanent,status,source_type,source_ref) VALUES($1,'chapter',$2,now(),true,'active','order',$3) ON CONFLICT(reader_id,entitlement_type,target_id) DO NOTHING`, readerID, snapshot.TargetID, o.OrderNo); err != nil {
		return ChapterResult{}, err
	}
	if _, err = executor.ExecContext(ctx, `UPDATE reader_purchase_orders SET recharge_coin_amount=$1,bonus_coin_amount=$2,paid_time=now() WHERE id=$3`, recharge, bonus, o.ID); err != nil {
		return ChapterResult{}, err
	}
	o.RechargeCoinAmount = strconv.FormatInt(recharge, 10)
	o.BonusCoinAmount = strconv.FormatInt(bonus, 10)
	o.Status = "paid"
	return ChapterResult{PurchaseStatus: "paid", Quote: q, Order: &o}, nil
}

func (r SQLRepository) BuyBook(ctx context.Context, readerID int64, snapshot novelcontract.PurchaseSnapshot, expected int64, key string) (Order, error) {
	executor, err := r.executor(ctx)
	if err != nil {
		return Order{}, err
	}
	var pid, price int64
	var name string
	if err = executor.QueryRowContext(ctx, `SELECT id,product_name,price_coin FROM commerce_products WHERE product_type='book' AND target_id=$1 AND sale_status='on_sale'`, snapshot.TargetID).Scan(&pid, &name, &price); err != nil {
		return Order{}, ErrProductUnavailable
	}
	if expected != price {
		return Order{}, ErrQuoteChanged
	}
	o, err := insertOrderTx(ctx, executor, readerID, "book", pid, "book", snapshot.TargetID, strconv.FormatInt(snapshot.BookID, 10), name, price, 0, 0, 0, key, orderNo("MBB", readerID))
	if err != nil {
		return Order{}, err
	}
	bonus, recharge, err := debitMixedTx(ctx, executor, readerID, price, o.ID, o.OrderNo, "book_purchase")
	if err != nil {
		return Order{}, err
	}
	if _, err = executor.ExecContext(ctx, `INSERT INTO commerce_entitlements(reader_id,entitlement_type,target_id,starts_at,permanent,status,source_type,source_ref) VALUES($1,'book',$2,now(),true,'active','order',$3) ON CONFLICT(reader_id,entitlement_type,target_id) DO NOTHING`, readerID, snapshot.TargetID, o.OrderNo); err != nil {
		return Order{}, err
	}
	if _, err = executor.ExecContext(ctx, `UPDATE reader_purchase_orders SET recharge_coin_amount=$1,bonus_coin_amount=$2,paid_time=now() WHERE id=$3`, recharge, bonus, o.ID); err != nil {
		return Order{}, err
	}
	o.RechargeCoinAmount = strconv.FormatInt(recharge, 10)
	o.BonusCoinAmount = strconv.FormatInt(bonus, 10)
	o.Status = "paid"
	return o, nil
}

func findOrderTx(ctx context.Context, tx transaction.DBTX, readerID int64, key string) (Order, error) {
	var o Order
	err := tx.QueryRowContext(ctx, `SELECT id::text,reader_id::text,order_no,order_type,COALESCE(product_id::text,''),product_type,COALESCE(target_id::text,''),COALESCE(book_id_snapshot::text,''),product_name_snapshot,price_coin_snapshot::text,COALESCE(chapter_word_count_snapshot::text,''),COALESCE(pricing_word_unit_snapshot::text,''),COALESCE(pricing_coin_unit_snapshot::text,''),recharge_coin_amount::text,bonus_coin_amount::text,status,idempotency_key,remark,paid_time,created_at,updated_at FROM reader_purchase_orders WHERE reader_id=$1 AND idempotency_key=$2`, readerID, key).Scan(&o.ID, &o.ReaderID, &o.OrderNo, &o.OrderType, &o.ProductID, &o.ProductType, &o.TargetID, &o.BookIDSnapshot, &o.ProductName, &o.PriceCoin, &o.ChapterWordCount, &o.PricingWordUnit, &o.PricingCoinUnit, &o.RechargeCoinAmount, &o.BonusCoinAmount, &o.Status, &o.IdempotencyKey, &o.Remark, &o.PaidTime, &o.CreateTime, &o.UpdateTime)
	return o, err
}
func insertOrderTx(ctx context.Context, tx transaction.DBTX, readerID int64, typ string, productID int64, productType string, targetID int64, bookID, name string, price int64, words, wordUnit int, coinUnit int64, key, order string) (Order, error) {
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
func debitMixedTx(ctx context.Context, tx transaction.DBTX, readerID, amount int64, orderID string, orderNo, biz string) (int64, int64, error) {
	return debitTx(ctx, tx, readerID, amount, orderID, orderNo, biz, true)
}

func debitRechargeOnlyTx(ctx context.Context, tx transaction.DBTX, readerID, amount int64, orderID string, orderNo, biz string) (int64, int64, error) {
	return debitTx(ctx, tx, readerID, amount, orderID, orderNo, biz, false)
}

func debitTx(ctx context.Context, tx transaction.DBTX, readerID, amount int64, orderID string, orderNo, biz string, useBonus bool) (int64, int64, error) {
	var bonusBalance, rechargeBalance int64
	if e := tx.QueryRowContext(ctx, `INSERT INTO reader_wallets(reader_id) VALUES($1) ON CONFLICT(reader_id) DO NOTHING`, readerID).Err(); e != nil {
		return 0, 0, e
	}
	if e := tx.QueryRowContext(ctx, `SELECT bonus_coin_balance,recharge_coin_balance FROM reader_wallets WHERE reader_id=$1 FOR UPDATE`, readerID).Scan(&bonusBalance, &rechargeBalance); e != nil {
		return 0, 0, e
	}
	bonus := amount
	if !useBonus {
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
