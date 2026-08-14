package adminrechargeorder

import (
	"context"
	"database/sql"
)

type SQLRepository struct{ DB *sql.DB }

const selectOrder = `SELECT o.id::text,o.reader_id::text,o.diamond_amount::text,COALESCE(o.product_id::text,''),a.username,o.order_no,o.source_type,o.price_usdt::text,o.provider,o.currency,o.token,o.network,COALESCE(o.gateway_trade_id,''),COALESCE(o.actual_amount::text,''),COALESCE(o.receive_address,''),COALESCE(o.payment_url,''),COALESCE(o.block_transaction_id,''),o.status,o.gateway_status,o.created_at,o.paid_time FROM reader_recharge_orders o JOIN reader_accounts a ON a.id=o.reader_id`

func scan(row interface{ Scan(...any) error }, o *Order) error {
	return row.Scan(&o.ID, &o.ReaderID, &o.DiamondAmount, &o.ProductID, &o.ReaderUsername, &o.OrderNo, &o.SourceType, &o.PriceUSDT, &o.Provider, &o.Currency, &o.Token, &o.Network, &o.GatewayTradeID, &o.ActualAmount, &o.ReceiveAddress, &o.PaymentURL, &o.BlockTransactionID, &o.Status, &o.GatewayStatus, &o.CreatedAt, &o.PaidAt)
}
func (r SQLRepository) List(ctx context.Context, keyword, status string, page, size int) ([]Order, int64, error) {
	var total int64
	where := ` WHERE ($1='' OR o.order_no ILIKE '%'||$1||'%' OR a.username ILIKE '%'||$1||'%') AND ($2='' OR o.status=$2)`
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM reader_recharge_orders o JOIN reader_accounts a ON a.id=o.reader_id`+where, keyword, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, selectOrder+where+` ORDER BY o.created_at DESC,o.id DESC LIMIT $3 OFFSET $4`, keyword, status, size, (page-1)*size)
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
	var o Order
	err := scan(r.DB.QueryRowContext(ctx, selectOrder+` WHERE o.id=$1`, id), &o)
	return o, err
}
