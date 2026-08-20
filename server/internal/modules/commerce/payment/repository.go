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

func (r SQLRepository) VerificationSnapshot(ctx context.Context, orderNo string) (VerificationSnapshot, error) {
	if r.DB == nil {
		return VerificationSnapshot{}, readerDependencyUnavailable
	}
	var snapshot VerificationSnapshot
	err := r.DB.QueryRowContext(ctx, `SELECT id,COALESCE(credential_ref,''),COALESCE(merchant_pid_snapshot,'') FROM reader_recharge_orders WHERE provider='epusdt' AND order_no=$1`, orderNo).Scan(&snapshot.OrderID, &snapshot.CredentialRef, &snapshot.MerchantPID)
	return snapshot, err
}

func (r SQLRepository) Process(ctx context.Context, c Callback) error {
	_, err := r.process(ctx, nil, c)
	return err
}

func (r SQLRepository) ProcessAttempt(ctx context.Context, attemptID int64, c Callback) (AttemptResult, error) {
	if attemptID <= 0 {
		return "", newProcessingError(FailureTransaction, ErrInvalidAttempt)
	}
	return r.process(ctx, &attemptID, c)
}

func (r SQLRepository) process(ctx context.Context, attemptID *int64, c Callback) (AttemptResult, error) {
	if c.Status != 2 || !strings.EqualFold(c.Token, "usdt") || c.OrderNo == "" || c.TradeID == "" || c.TransactionID == "" || c.ActualAmount == "" || c.ReceiveAddress == "" {
		return "", newProcessingError(FailureSnapshotMismatch, ErrRejected)
	}
	if r.DB == nil || r.Accounts == nil {
		return "", newProcessingError(FailureDependency, readercontract.ErrUnavailable)
	}
	var expectedReaderID int64
	if err := r.DB.QueryRowContext(ctx, `SELECT reader_id FROM reader_recharge_orders WHERE provider='epusdt' AND order_no=$1`, c.OrderNo).Scan(&expectedReaderID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", newProcessingError(FailureUnknownOrder, ErrRejected)
		}
		return "", newProcessingError(FailureDependency, readerDependencyUnavailable)
	}
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return "", newProcessingError(FailureTransaction, ErrRejected)
	}
	defer tx.Rollback()
	if e = transaction.WithExisting(ctx, tx, func(txCtx context.Context) error {
		_, lockErr := r.Accounts.LockAccount(txCtx, expectedReaderID)
		return lockErr
	}); e != nil {
		if errors.Is(e, readercontract.ErrUnavailable) || errors.Is(e, readercontract.ErrTimeout) || errors.Is(e, readercontract.ErrUnknown) {
			return "", newProcessingError(FailureDependency, readercontract.ErrUnavailable)
		}
		return "", newProcessingError(FailureSnapshotMismatch, ErrRejected)
	}
	var orderID, readerID, diamonds int64
	var price, status, provider, currency, token, network string
	var oldTx, oldTrade, oldActual, oldAddress sql.NullString
	if e = tx.QueryRowContext(ctx, `SELECT id,reader_id,diamond_amount,price_usdt::text,status,block_transaction_id,gateway_trade_id,actual_amount::text,receive_address,provider,currency,token,network FROM reader_recharge_orders WHERE provider='epusdt' AND order_no=$1 FOR UPDATE`, c.OrderNo).Scan(&orderID, &readerID, &diamonds, &price, &status, &oldTx, &oldTrade, &oldActual, &oldAddress, &provider, &currency, &token, &network); e != nil {
		if errors.Is(e, sql.ErrNoRows) {
			return "", newProcessingError(FailureUnknownOrder, ErrRejected)
		}
		return "", newProcessingError(FailureTransaction, ErrRejected)
	}
	if readerID != expectedReaderID {
		return "", newProcessingError(FailureSnapshotMismatch, ErrRejected)
	}
	baseSnapshotMatches := provider == "epusdt" && currency == "usd" && strings.EqualFold(token, "usdt") && network == "tron" &&
		decimalEqual(price, c.Amount) && positiveDecimal(c.ActualAmount) &&
		(!oldTrade.Valid || strings.TrimSpace(oldTrade.String) == "" || oldTrade.String == c.TradeID) &&
		(!oldActual.Valid || strings.TrimSpace(oldActual.String) == "" || decimalEqual(oldActual.String, c.ActualAmount)) &&
		(!oldAddress.Valid || strings.TrimSpace(oldAddress.String) == "" || oldAddress.String == c.ReceiveAddress)
	if status == "paid" {
		exactMatch := baseSnapshotMatches && oldTrade.Valid && oldTrade.String == c.TradeID &&
			oldActual.Valid && decimalEqual(oldActual.String, c.ActualAmount) && oldAddress.Valid && oldAddress.String == c.ReceiveAddress &&
			oldTx.Valid && oldTx.String == c.TransactionID
		if !exactMatch {
			return "", newProcessingError(FailureSnapshotMismatch, ErrRejected)
		}
		if attemptID == nil {
			return ResultIdempotent, nil
		}
		snapshot := SnapshotFromCallback(c)
		if e = finalizeAttempt(ctx, tx, *attemptID, AttemptCompletion{Result: ResultIdempotent, ResponseStatus: 200, SignatureValid: true, OrderID: &orderID, Snapshot: &snapshot}); e != nil {
			return "", newProcessingError(FailureTransaction, ErrRejected)
		}
		if e = tx.Commit(); e != nil {
			return "", newProcessingError(FailureTransaction, ErrRejected)
		}
		return ResultIdempotent, nil
	}
	if !allowedCallbackStatus(status) || !baseSnapshotMatches {
		return "", newProcessingError(FailureSnapshotMismatch, ErrRejected)
	}
	var usedTrade, usedTransaction int64
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE gateway_trade_id=$1),count(*) FILTER (WHERE block_transaction_id=$2) FROM reader_recharge_orders WHERE id<>$3`, c.TradeID, c.TransactionID, orderID).Scan(&usedTrade, &usedTransaction); e != nil {
		return "", newProcessingError(FailureTransaction, ErrRejected)
	}
	if usedTrade > 0 || usedTransaction > 0 {
		return "", newProcessingError(FailureReplay, ErrRejected)
	}
	key := "epusdt_recharge:" + c.OrderNo
	biz := new(string)
	*biz = strconv64(orderID)
	ledgerNo := "R" + strconv64(orderID)
	ledger, e := wallet.MutateTx(ctx, tx, wallet.Mutation{ReaderID: readerID, LedgerNo: ledgerNo, BizType: "epusdt_recharge", BizID: biz, OrderNo: &c.OrderNo, Direction: "income", CoinType: "recharge", Amount: diamonds, Remark: strPtr("USDT充值"), IdempotencyKey: &key})
	if e != nil {
		return "", newProcessingError(FailureTransaction, ErrRejected)
	}
	if e = transaction.WithExisting(ctx, tx, func(txCtx context.Context) error {
		return invitereward.GrantFirstRechargeTx(txCtx, tx, r.Invites, readerID)
	}); e != nil {
		if errors.Is(e, readercontract.ErrUnavailable) {
			return "", newProcessingError(FailureDependency, readercontract.ErrUnavailable)
		}
		return "", newProcessingError(FailureTransaction, ErrRejected)
	}
	if _, e = tx.ExecContext(ctx, `UPDATE reader_recharge_orders SET gateway_trade_id=$1,actual_amount=$2,receive_address=$3,block_transaction_id=$4,gateway_status=2,status='paid',wallet_ledger_id=$5,active_reader_id=NULL,failure_code=NULL,failure_message=NULL,paid_time=now(),updated_at=now() WHERE id=$6`, c.TradeID, c.ActualAmount, c.ReceiveAddress, c.TransactionID, ledger.ID, orderID); e != nil {
		return "", newProcessingError(FailureTransaction, ErrRejected)
	}
	if attemptID != nil {
		snapshot := SnapshotFromCallback(c)
		if e = finalizeAttempt(ctx, tx, *attemptID, AttemptCompletion{Result: ResultSuccess, ResponseStatus: 200, SignatureValid: true, OrderID: &orderID, Snapshot: &snapshot}); e != nil {
			return "", newProcessingError(FailureTransaction, ErrRejected)
		}
	} else {
		hash := sha256.Sum256([]byte(Canonical(c.Fields)))
		_, e = tx.ExecContext(ctx, `INSERT INTO reader_payment_callback_logs(provider,recharge_order_id,merchant_order_no,gateway_trade_id,payload_hash,signature_valid,processing_result,response_status,response_body,request_time) VALUES('epusdt',$1,$2,$3,$4,true,'success',200,'success',now())`, orderID, c.OrderNo, c.TradeID, hex.EncodeToString(hash[:]))
		if e != nil {
			return "", newProcessingError(FailureTransaction, ErrRejected)
		}
	}
	if e = tx.Commit(); e != nil {
		return "", newProcessingError(FailureTransaction, ErrRejected)
	}
	return ResultSuccess, nil
}

func allowedCallbackStatus(status string) bool {
	switch status {
	case "pending", "gateway_unknown", "superseded", "expired", "callback_exception":
		return true
	default:
		return false
	}
}

func positiveDecimal(value string) bool {
	parsed, ok := new(big.Rat).SetString(value)
	return ok && parsed.Sign() > 0
}

func decimalEqual(a, b string) bool {
	ra, oka := new(big.Rat).SetString(a)
	rb, okb := new(big.Rat).SetString(b)
	return oka && okb && ra.Cmp(rb) == 0
}
func strPtr(v string) *string  { return &v }
func strconv64(v int64) string { return fmt.Sprintf("%d", v) }
