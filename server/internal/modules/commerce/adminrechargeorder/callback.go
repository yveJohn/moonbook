package adminrechargeorder

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type CallbackSnapshot struct {
	OrderNo        string `json:"order_id,omitempty"`
	TradeID        string `json:"trade_id,omitempty"`
	Amount         string `json:"amount,omitempty"`
	ActualAmount   string `json:"actual_amount,omitempty"`
	ReceiveAddress string `json:"receive_address,omitempty"`
	Token          string `json:"token,omitempty"`
	TransactionID  string `json:"block_transaction_id,omitempty"`
	Status         string `json:"status,omitempty"`
}

type CallbackLog struct {
	ID, OrderID, Provider, MerchantOrderNo, GatewayTradeID                  string
	PayloadHash, ProcessingResult, FailureCode, FailureReason, ResponseBody string
	RequestID, TraceID, SourceType, SignatureStatus                         string
	SignatureValid, PayloadTruncated, Interrupted                           bool
	ResponseStatus, PayloadBytes                                            int
	RequestTime, CreatedAt                                                  time.Time
	CompletedAt                                                             *time.Time
	Snapshot                                                                CallbackSnapshot
}

func (r SQLRepository) ListCallbacks(ctx context.Context, filter CallbackFilter) ([]CallbackLog, int64, error) {
	where := ` WHERE
($1='' OR l.merchant_order_no ILIKE '%'||$1||'%' OR l.gateway_trade_id ILIKE '%'||$1||'%')
AND ($2='' OR l.processing_result=$2)
AND ($3='' OR ($3='valid' AND l.signature_valid=true) OR ($3='invalid' AND l.failure_code='SIGNATURE_INVALID') OR ($3='not_checked' AND l.signature_valid=false AND COALESCE(l.failure_code,'')<>'SIGNATURE_INVALID'))
AND ($4='' OR l.failure_code=$4)
AND ($5::integer IS NULL OR l.response_status=$5)
AND ($6::timestamptz IS NULL OR l.request_time >= $6)
AND ($7::timestamptz IS NULL OR l.request_time <= $7)`
	args := []any{filter.Keyword, filter.ProcessingResult, filter.SignatureStatus, filter.FailureCode, filter.ResponseStatus, filter.StartTime, filter.EndTime}
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM reader_payment_callback_logs l`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, `
SELECT l.id::text,COALESCE(l.recharge_order_id::text,''),l.provider,l.merchant_order_no,l.gateway_trade_id,
       l.payload_hash,l.signature_valid,l.processing_result,COALESCE(l.failure_code,''),l.failure_reason,
       l.response_status,l.response_body,l.request_time,l.created_at,COALESCE(l.request_id,''),COALESCE(l.trace_id,''),
       COALESCE(l.payload_bytes,0),COALESCE(l.payload_truncated,false),l.completed_at,l.source_type,l.payload_snapshot
FROM reader_payment_callback_logs l`+where+`
ORDER BY l.request_time DESC,l.id DESC LIMIT $8 OFFSET $9`, append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]CallbackLog, 0)
	for rows.Next() {
		item, err := scanCallback(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r SQLRepository) GetCallback(ctx context.Context, id int64) (CallbackLog, error) {
	row := r.DB.QueryRowContext(ctx, `
SELECT l.id::text,COALESCE(l.recharge_order_id::text,''),l.provider,l.merchant_order_no,l.gateway_trade_id,
       l.payload_hash,l.signature_valid,l.processing_result,COALESCE(l.failure_code,''),l.failure_reason,
       l.response_status,l.response_body,l.request_time,l.created_at,COALESCE(l.request_id,''),COALESCE(l.trace_id,''),
       COALESCE(l.payload_bytes,0),COALESCE(l.payload_truncated,false),l.completed_at,l.source_type,l.payload_snapshot
FROM reader_payment_callback_logs l WHERE l.id=$1`, id)
	return scanCallback(row)
}

type callbackScanner interface{ Scan(...any) error }

func scanCallback(scanner callbackScanner) (CallbackLog, error) {
	var item CallbackLog
	var snapshot []byte
	var completed sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.OrderID, &item.Provider, &item.MerchantOrderNo, &item.GatewayTradeID,
		&item.PayloadHash, &item.SignatureValid, &item.ProcessingResult, &item.FailureCode, &item.FailureReason,
		&item.ResponseStatus, &item.ResponseBody, &item.RequestTime, &item.CreatedAt, &item.RequestID, &item.TraceID,
		&item.PayloadBytes, &item.PayloadTruncated, &completed, &item.SourceType, &snapshot,
	); err != nil {
		return CallbackLog{}, err
	}
	if completed.Valid {
		value := completed.Time
		item.CompletedAt = &value
	}
	if len(snapshot) > 0 {
		if err := json.Unmarshal(snapshot, &item.Snapshot); err != nil {
			return CallbackLog{}, err
		}
	}
	item.SignatureStatus = "not_checked"
	if item.SignatureValid {
		item.SignatureStatus = "valid"
	} else if item.FailureCode == "SIGNATURE_INVALID" {
		item.SignatureStatus = "invalid"
	}
	return item, nil
}
