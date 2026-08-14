package adminrechargeorder

import (
	"context"
	"time"
)

type CallbackLog struct {
	ID, OrderID, Provider, MerchantOrderNo, GatewayTradeID     string
	PayloadHash, ProcessingResult, FailureReason, ResponseBody string
	SignatureValid                                             bool
	ResponseStatus                                             int
	RequestTime, CreatedAt                                     time.Time
}

func (r SQLRepository) ListCallbacks(ctx context.Context, keyword, result string, page, size int) ([]CallbackLog, int64, error) {
	where := ` WHERE ($1='' OR l.merchant_order_no ILIKE '%'||$1||'%' OR l.gateway_trade_id ILIKE '%'||$1||'%') AND ($2='' OR l.processing_result=$2)`
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM reader_payment_callback_logs l`+where, keyword, result).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT l.id::text,COALESCE(l.recharge_order_id::text,''),l.provider,l.merchant_order_no,l.gateway_trade_id,l.payload_hash,l.signature_valid,l.processing_result,l.failure_reason,l.response_status,l.response_body,l.request_time,l.created_at FROM reader_payment_callback_logs l`+where+` ORDER BY l.created_at DESC,l.id DESC LIMIT $3 OFFSET $4`, keyword, result, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]CallbackLog, 0)
	for rows.Next() {
		var v CallbackLog
		if err := rows.Scan(&v.ID, &v.OrderID, &v.Provider, &v.MerchantOrderNo, &v.GatewayTradeID, &v.PayloadHash, &v.SignatureValid, &v.ProcessingResult, &v.FailureReason, &v.ResponseStatus, &v.ResponseBody, &v.RequestTime, &v.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, v)
	}
	return items, total, rows.Err()
}

func (r SQLRepository) GetCallback(ctx context.Context, id int64) (CallbackLog, error) {
	var v CallbackLog
	err := r.DB.QueryRowContext(ctx, `SELECT l.id::text,COALESCE(l.recharge_order_id::text,''),l.provider,l.merchant_order_no,l.gateway_trade_id,l.payload_hash,l.signature_valid,l.processing_result,l.failure_reason,l.response_status,l.response_body,l.request_time,l.created_at FROM reader_payment_callback_logs l WHERE l.id=$1`, id).Scan(&v.ID, &v.OrderID, &v.Provider, &v.MerchantOrderNo, &v.GatewayTradeID, &v.PayloadHash, &v.SignatureValid, &v.ProcessingResult, &v.FailureReason, &v.ResponseStatus, &v.ResponseBody, &v.RequestTime, &v.CreatedAt)
	return v, err
}
