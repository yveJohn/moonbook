package adminorder

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

type SQLRepository struct{ DB *sql.DB }

const orderSelect = `SELECT o.id::text,o.reader_id::text,a.username,o.order_no,o.order_type,COALESCE(o.product_id::text,''),o.product_type,COALESCE(o.target_id::text,''),COALESCE(o.book_id_snapshot::text,''),o.product_name_snapshot,o.price_coin_snapshot::text,COALESCE(o.chapter_word_count_snapshot::text,''),COALESCE(o.pricing_word_unit_snapshot::text,''),COALESCE(o.pricing_coin_unit_snapshot::text,''),o.recharge_coin_amount::text,o.bonus_coin_amount::text,o.status,o.idempotency_key,COALESCE(o.remark,''),o.paid_time,o.created_at,o.updated_at FROM reader_purchase_orders o JOIN reader_accounts a ON a.id=o.reader_id`

func scan(row interface{ Scan(...any) error }, o *Order) error {
	return row.Scan(&o.ID, &o.ReaderID, &o.ReaderUsername, &o.OrderNo, &o.OrderType, &o.ProductID, &o.ProductType, &o.TargetID, &o.BookIDSnapshot, &o.ProductName, &o.PriceCoin, &o.ChapterWordCount, &o.PricingWordUnit, &o.PricingCoinUnit, &o.RechargeCoinAmount, &o.BonusCoinAmount, &o.Status, &o.IdempotencyKey, &o.Remark, &o.PaidAt, &o.CreatedAt, &o.UpdatedAt)
}

func (r SQLRepository) List(ctx context.Context, keyword, orderType, status string, page, size int) ([]Order, int64, error) {
	where := ` WHERE ($1='' OR o.order_no ILIKE '%'||$1||'%' OR a.username ILIKE '%'||$1||'%') AND ($2='' OR o.order_type=$2) AND ($3='' OR o.status=$3)`
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM reader_purchase_orders o JOIN reader_accounts a ON a.id=o.reader_id`+where, keyword, orderType, status).Scan(&total); err != nil {
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
	err := scan(r.DB.QueryRowContext(ctx, orderSelect+` WHERE o.id=$1`, id), &o)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "消费订单不存在")
	}
	return o, err
}
