package payment

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/invitereward"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/wallet"
	readercontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

var ErrRejected = errors.New("payment callback rejected")

type SQLRepository struct {
	DB       *sql.DB
	Invites  readercontract.InviteRelationReader
	Accounts readercontract.AccountLocker
}

func (r SQLRepository) Process(ctx context.Context, c Callback) error {
	if c.Status != 2 || !strings.EqualFold(c.Token, "usdt") || c.OrderNo == "" || c.TransactionID == "" {
		return ErrRejected
	}
	if r.Accounts == nil {
		return readercontract.ErrUnavailable
	}
	var expectedReaderID int64
	if e := r.DB.QueryRowContext(ctx, `SELECT reader_id FROM reader_recharge_orders WHERE order_no=$1`, c.OrderNo).Scan(&expectedReaderID); e != nil {
		return ErrRejected
	}
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = transaction.WithExisting(ctx, tx, func(txCtx context.Context) error {
		_, lockErr := r.Accounts.LockAccount(txCtx, expectedReaderID)
		return lockErr
	}); e != nil {
		return e
	}
	var orderID, readerID, diamonds int64
	var price string
	var status string
	var oldTx sql.NullString
	if e = tx.QueryRowContext(ctx, `SELECT id,reader_id,diamond_amount,price_usdt::text,status,block_transaction_id FROM reader_recharge_orders WHERE order_no=$1 FOR UPDATE`, c.OrderNo).Scan(&orderID, &readerID, &diamonds, &price, &status, &oldTx); e != nil {
		return ErrRejected
	}
	if readerID != expectedReaderID {
		return ErrRejected
	}
	if status == "paid" {
		if oldTx.Valid && oldTx.String == c.TransactionID {
			return nil
		}
		return ErrRejected
	}
	if !decimalEqual(price, c.Amount) {
		return ErrRejected
	}
	var used int64
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM reader_recharge_orders WHERE block_transaction_id=$1 AND id<>$2`, c.TransactionID, orderID).Scan(&used); e != nil || used > 0 {
		return ErrRejected
	}
	key := "epusdt_recharge:" + c.OrderNo
	biz := new(string)
	*biz = strconv64(orderID)
	ledgerNo := "R" + strconv64(orderID)
	ledger, e := wallet.MutateTx(ctx, tx, wallet.Mutation{ReaderID: readerID, LedgerNo: ledgerNo, BizType: "epusdt_recharge", BizID: biz, OrderNo: &c.OrderNo, Direction: "income", CoinType: "recharge", Amount: diamonds, Remark: strPtr("USDT充值"), IdempotencyKey: &key})
	if e != nil {
		return e
	}
	if e = transaction.WithExisting(ctx, tx, func(txCtx context.Context) error {
		return invitereward.GrantFirstRechargeTx(txCtx, tx, r.Invites, readerID)
	}); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE reader_recharge_orders SET gateway_trade_id=$1,actual_amount=$2,receive_address=$3,block_transaction_id=$4,gateway_status=2,status='paid',wallet_ledger_id=$5,paid_time=now(),updated_at=now() WHERE id=$6`, c.TradeID, c.ActualAmount, c.ReceiveAddress, c.TransactionID, ledger.ID, orderID); e != nil {
		return e
	}
	hash := sha256.Sum256([]byte(Canonical(c.Fields)))
	_, e = tx.ExecContext(ctx, `INSERT INTO reader_payment_callback_logs(provider,recharge_order_id,merchant_order_no,gateway_trade_id,payload_hash,signature_valid,processing_result,response_status,response_body,request_time) VALUES('epusdt',$1,$2,$3,$4,true,'success',200,'success',now())`, orderID, c.OrderNo, c.TradeID, hex.EncodeToString(hash[:]))
	if e != nil {
		return e
	}
	return tx.Commit()
}
func decimalEqual(a, b string) bool {
	ra, oka := new(big.Rat).SetString(a)
	rb, okb := new(big.Rat).SetString(b)
	return oka && okb && ra.Cmp(rb) == 0
}
func strPtr(v string) *string  { return &v }
func strconv64(v int64) string { return fmt.Sprintf("%d", v) }
