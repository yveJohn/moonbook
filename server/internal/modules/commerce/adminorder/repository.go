package adminorder

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/invitereward"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/wallet"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

type SQLRepository struct{ DB *sql.DB }

const orderSelect = `SELECT o.id::text,o.reader_id::text,p.username,o.order_no,o.order_type,COALESCE(o.product_id::text,''),o.product_type,COALESCE(o.target_id::text,''),COALESCE(o.book_id_snapshot::text,''),o.product_name_snapshot,o.price_coin_snapshot::text,COALESCE(o.chapter_word_count_snapshot::text,''),COALESCE(o.pricing_word_unit_snapshot::text,''),COALESCE(o.pricing_coin_unit_snapshot::text,''),o.recharge_coin_amount::text,o.bonus_coin_amount::text,o.status,o.idempotency_key,COALESCE(o.remark,''),o.paid_time,o.created_at,o.updated_at FROM reader_purchase_orders o JOIN commerce_reader_search_projection p ON p.reader_id=o.reader_id`

func scan(row interface{ Scan(...any) error }, o *Order) error {
	return row.Scan(&o.ID, &o.ReaderID, &o.ReaderUsername, &o.OrderNo, &o.OrderType, &o.ProductID, &o.ProductType, &o.TargetID, &o.BookIDSnapshot, &o.ProductName, &o.PriceCoin, &o.ChapterWordCount, &o.PricingWordUnit, &o.PricingCoinUnit, &o.RechargeCoinAmount, &o.BonusCoinAmount, &o.Status, &o.IdempotencyKey, &o.Remark, &o.PaidAt, &o.CreatedAt, &o.UpdatedAt)
}

func (r SQLRepository) List(ctx context.Context, keyword, orderType, status string, page, size int) ([]Order, int64, error) {
	where := ` WHERE ($1='' OR o.order_no ILIKE '%'||$1||'%' OR p.username ILIKE '%'||$1||'%') AND ($2='' OR o.order_type=$2) AND ($3='' OR o.status=$3)`
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM reader_purchase_orders o JOIN commerce_reader_search_projection p ON p.reader_id=o.reader_id`+where, keyword, orderType, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, orderSelect+where+` ORDER BY o.created_at DESC,o.id DESC LIMIT $4 OFFSET $5`, keyword, orderType, status, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]Order, 0)
	for rows.Next() {
		var o Order
		if err := scan(rows, &o); err != nil {
			return nil, 0, err
		}
		items = append(items, o)
	}
	return items, total, rows.Err()
}

func (r SQLRepository) Get(ctx context.Context, id int64) (Order, error) {
	if id <= 0 {
		return Order{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串")
	}
	var o Order
	err := scan(transaction.Executor(ctx, r.DB).QueryRowContext(ctx, orderSelect+` WHERE o.id=$1`, id), &o)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "消费订单不存在")
	}
	return o, err
}

func (r SQLRepository) CreateMockRecharge(ctx context.Context, in MockRechargeInput, operatorID int64) (Order, error) {
	readerID, err := strconv.ParseInt(in.ReaderID, 10, 64)
	if err != nil {
		return Order{}, err
	}
	amount, err := strconv.ParseInt(in.RechargeCoinAmount, 10, 64)
	if err != nil {
		return Order{}, err
	}
	executor := transaction.Executor(ctx, r.DB)
	if executor == r.DB {
		return Order{}, transaction.ErrNoTransaction
	}
	key := "mock_recharge_create:" + in.RequestID
	orderNo := fmt.Sprintf("MR-%d-%d", readerID, time.Now().UnixNano())
	var orderID int64
	err = executor.QueryRowContext(ctx, `INSERT INTO reader_purchase_orders(order_no,reader_id,order_type,product_type,product_name_snapshot,price_coin_snapshot,recharge_coin_amount,bonus_coin_amount,status,idempotency_key,remark,operator_id) VALUES($1,$2,'mock_recharge','recharge','模拟充值',$3,$3,0,'pending',$4,$5,$6) ON CONFLICT(reader_id,idempotency_key) DO UPDATE SET updated_at=reader_purchase_orders.updated_at RETURNING id`, orderNo, readerID, amount, key, in.Remark, operatorID).Scan(&orderID)
	if err != nil {
		return Order{}, err
	}
	return r.Get(ctx, orderID)
}

func (r SQLRepository) ConfirmMockRecharge(ctx context.Context, orderID, operatorID int64) (Order, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()
	var readerID, amount int64
	var orderNo, orderType, status, remark string
	err = tx.QueryRowContext(ctx, `SELECT reader_id,order_no,order_type,recharge_coin_amount,status,COALESCE(remark,'') FROM reader_purchase_orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&readerID, &orderNo, &orderType, &amount, &status, &remark)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "模拟充值订单不存在")
	}
	if err != nil {
		return Order{}, err
	}
	if orderType != "mock_recharge" {
		return Order{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "订单类型不支持模拟充值确认")
	}
	if status == "paid" {
		if err = tx.Commit(); err != nil {
			return Order{}, err
		}
		return r.Get(ctx, orderID)
	}
	if status != "pending" {
		return Order{}, apperror.New(apperror.CodeConflict, http.StatusConflict, "订单状态不支持确认")
	}
	orderIDText := strconv.FormatInt(orderID, 10)
	confirmKey := "mock_recharge_confirm:" + orderNo
	if _, err = wallet.MutateTx(ctx, tx, wallet.Mutation{ReaderID: readerID, LedgerNo: "MR-" + orderIDText, BizType: "mock_recharge", BizID: &orderIDText, OrderNo: &orderNo, Direction: "income", CoinType: "recharge", Amount: amount, Remark: &remark, IdempotencyKey: &confirmKey}); err != nil {
		return Order{}, err
	}
	if err = invitereward.GrantFirstRechargeTx(ctx, tx, readerID); err != nil {
		return Order{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE reader_purchase_orders SET status='paid',operator_id=$2,paid_time=now(),updated_at=now() WHERE id=$1`, orderID, operatorID); err != nil {
		return Order{}, err
	}
	if err = tx.Commit(); err != nil {
		return Order{}, err
	}
	return r.Get(ctx, orderID)
}
