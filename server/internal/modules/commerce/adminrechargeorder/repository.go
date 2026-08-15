package adminrechargeorder

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/invitereward"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/wallet"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

type SQLRepository struct{ DB *sql.DB }

const selectOrder = `SELECT o.id::text,o.reader_id::text,o.diamond_amount::text,COALESCE(o.product_id::text,''),p.username,o.order_no,o.source_type,o.price_usdt::text,o.provider,o.currency,o.token,o.network,COALESCE(o.gateway_trade_id,''),COALESCE(o.actual_amount::text,''),COALESCE(o.receive_address,''),COALESCE(o.payment_url,''),COALESCE(o.block_transaction_id,''),o.status,o.gateway_status,o.created_at,o.paid_time FROM reader_recharge_orders o JOIN commerce_reader_search_projection p ON p.reader_id=o.reader_id`

func scan(row interface{ Scan(...any) error }, o *Order) error {
	return row.Scan(&o.ID, &o.ReaderID, &o.DiamondAmount, &o.ProductID, &o.ReaderUsername, &o.OrderNo, &o.SourceType, &o.PriceUSDT, &o.Provider, &o.Currency, &o.Token, &o.Network, &o.GatewayTradeID, &o.ActualAmount, &o.ReceiveAddress, &o.PaymentURL, &o.BlockTransactionID, &o.Status, &o.GatewayStatus, &o.CreatedAt, &o.PaidAt)
}
func (r SQLRepository) List(ctx context.Context, keyword, status string, page, size int) ([]Order, int64, error) {
	var total int64
	where := ` WHERE ($1='' OR o.order_no ILIKE '%'||$1||'%' OR p.username ILIKE '%'||$1||'%') AND ($2='' OR o.status=$2)`
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM reader_recharge_orders o JOIN commerce_reader_search_projection p ON p.reader_id=o.reader_id`+where, keyword, status).Scan(&total); err != nil {
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
	err := scan(transaction.Executor(ctx, r.DB).QueryRowContext(ctx, selectOrder+` WHERE o.id=$1`, id), &o)
	return o, err
}

func (r SQLRepository) ReaderID(ctx context.Context, id int64) (int64, error) {
	if id <= 0 {
		return 0, apperror.New(apperror.CodeInvalidArgument, 400, "ID必须是正整数字符串")
	}
	var readerID int64
	if err := r.DB.QueryRowContext(ctx, `SELECT reader_id FROM reader_recharge_orders WHERE id=$1`, id).Scan(&readerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, apperror.New(apperror.CodeNotFound, 404, "充值订单不存在")
		}
		return 0, err
	}
	return readerID, nil
}

func (r SQLRepository) ManualPay(ctx context.Context, id, expectedReaderID int64, in ManualPayInput) (Order, error) {
	if err := validateManualPay(id, &in); err != nil {
		return Order{}, err
	}
	executor := transaction.Executor(ctx, r.DB)
	if executor == r.DB {
		return Order{}, transaction.ErrNoTransaction
	}
	var orderID, readerID, diamonds int64
	var orderNo, status, price string
	if err := executor.QueryRowContext(ctx, `SELECT id,reader_id,diamond_amount,order_no,price_usdt::text,status FROM reader_recharge_orders WHERE id=$1 AND reader_id=$2 FOR UPDATE`, id, expectedReaderID).Scan(&orderID, &readerID, &diamonds, &orderNo, &price, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Order{}, apperror.New(apperror.CodeNotFound, 404, "充值订单不存在")
		}
		return Order{}, err
	}
	key := fmt.Sprintf("manual_recharge:%d:%s", orderID, in.RequestID)
	var existingID int64
	if err := executor.QueryRowContext(ctx, `SELECT id FROM reader_wallet_ledgers WHERE reader_id=$1 AND idempotency_key=$2`, readerID, key).Scan(&existingID); err == nil {
		var out Order
		if err = scan(executor.QueryRowContext(ctx, selectOrder+` WHERE o.id=$1`, orderID), &out); err != nil {
			return Order{}, err
		}
		return out, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Order{}, err
	}
	if status == "paid" {
		return Order{}, apperror.New(apperror.CodeConflict, 409, "充值订单已支付")
	}
	switch status {
	case "pending", "gateway_unknown", "callback_exception", "create_failed":
	default:
		return Order{}, apperror.New(apperror.CodeConflict, 409, "当前订单状态不允许人工补单")
	}
	if in.GatewayTradeID != "" {
		var used int64
		if err := executor.QueryRowContext(ctx, `SELECT count(*) FROM reader_recharge_orders WHERE gateway_trade_id=$1 AND id<>$2`, in.GatewayTradeID, orderID).Scan(&used); err != nil {
			return Order{}, err
		}
		if used > 0 {
			return Order{}, apperror.New(apperror.CodeConflict, 409, "网关交易号已用于其他订单")
		}
	}
	bizID := strconv.FormatInt(orderID, 10)
	ledgerNo := fmt.Sprintf("MR-%d-%d", orderID, time.Now().UnixNano())
	ledger, err := wallet.MutateTx(ctx, executor, wallet.Mutation{ReaderID: readerID, LedgerNo: ledgerNo, BizType: "manual_recharge", BizID: &bizID, OrderNo: &orderNo, Direction: "income", CoinType: "recharge", Amount: diamonds, Remark: &in.Remark, IdempotencyKey: &key})
	if err != nil {
		return Order{}, err
	}
	if err = invitereward.GrantFirstRechargeTx(ctx, executor, readerID); err != nil {
		return Order{}, err
	}
	if _, err = executor.ExecContext(ctx, `UPDATE reader_recharge_orders SET gateway_trade_id=NULLIF($1,''),actual_amount=$2,status='paid',wallet_ledger_id=$3,paid_time=now(),updated_at=now() WHERE id=$4`, in.GatewayTradeID, in.ActualAmount, ledger.ID, orderID); err != nil {
		return Order{}, err
	}
	hash := sha256.Sum256([]byte(strings.Join([]string{orderNo, in.RequestID, in.GatewayTradeID, in.ActualAmount, in.Remark}, "|")))
	if _, err = executor.ExecContext(ctx, `INSERT INTO reader_payment_callback_logs(provider,recharge_order_id,merchant_order_no,gateway_trade_id,payload_hash,signature_valid,processing_result,failure_reason,response_status,response_body,request_time) VALUES('manual',$1,$2,$3,$4,false,'manual_success','',200,'success',now())`, orderID, orderNo, in.GatewayTradeID, hex.EncodeToString(hash[:])); err != nil {
		return Order{}, err
	}
	var out Order
	if err = scan(executor.QueryRowContext(ctx, selectOrder+` WHERE o.id=$1`, orderID), &out); err != nil {
		return Order{}, err
	}
	return out, nil
}

func validateManualPay(id int64, in *ManualPayInput) error {
	if in == nil {
		return apperror.New(apperror.CodeInvalidArgument, 400, "人工补单参数无效")
	}
	in.RequestID = strings.TrimSpace(in.RequestID)
	in.GatewayTradeID = strings.TrimSpace(in.GatewayTradeID)
	in.ActualAmount = strings.TrimSpace(in.ActualAmount)
	in.Remark = strings.TrimSpace(in.Remark)
	if id <= 0 || in.RequestID == "" || len(in.RequestID) > 64 || len(in.GatewayTradeID) > 64 || in.Remark == "" || len([]rune(in.Remark)) > 255 || !positiveDecimal(in.ActualAmount) {
		return apperror.New(apperror.CodeInvalidArgument, 400, "人工补单参数无效")
	}
	return nil
}

func positiveDecimal(value string) bool {
	if !regexp.MustCompile(`^[0-9]+(\.[0-9]{1,8})?$`).MatchString(value) {
		return false
	}
	r, ok := new(big.Rat).SetString(value)
	return ok && r.Sign() > 0
}
