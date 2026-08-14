package adminwallet

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

type SQLRepository struct{ DB *sql.DB }

var ErrInvalidAdjustment = errors.New("invalid wallet adjustment")

func (r SQLRepository) ListWallets(ctx context.Context, keyword string, page, size int) ([]Wallet, int64, error) {
	where := ` WHERE ($1='' OR a.username ILIKE '%'||$1||'%' OR a.nickname ILIKE '%'||$1||'%')`
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM reader_accounts a LEFT JOIN reader_wallets w ON w.reader_id=a.id`+where, keyword).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT a.id::text,a.username,COALESCE(w.recharge_coin_balance,0)::text,COALESCE(w.bonus_coin_balance,0)::text,COALESCE(w.total_recharge_coin_income,0)::text,COALESCE(w.total_bonus_coin_income,0)::text,COALESCE(w.total_recharge_coin_expense,0)::text,COALESCE(w.total_bonus_coin_expense,0)::text FROM reader_accounts a LEFT JOIN reader_wallets w ON w.reader_id=a.id`+where+` ORDER BY a.id DESC LIMIT $2 OFFSET $3`, keyword, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]Wallet, 0)
	for rows.Next() {
		var v Wallet
		if err := rows.Scan(&v.ReaderID, &v.ReaderUsername, &v.RechargeBalance, &v.BonusBalance, &v.TotalRechargeIncome, &v.TotalBonusIncome, &v.TotalRechargeExpense, &v.TotalBonusExpense); err != nil {
			return nil, 0, err
		}
		items = append(items, v)
	}
	return items, total, rows.Err()
}
func (r SQLRepository) ListLedgers(ctx context.Context, readerID int64, coin string, page, size int) ([]Ledger, int64, error) {
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1 AND ($2='' OR coin_type=$2)`, readerID, coin).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT id::text,reader_id::text,ledger_no,biz_type,direction,coin_type,amount::text,balance_before::text,balance_after::text,COALESCE(biz_id,''),COALESCE(order_no,''),COALESCE(remark,''),created_at::text FROM reader_wallet_ledgers WHERE reader_id=$1 AND ($2='' OR coin_type=$2) ORDER BY created_at DESC,id DESC LIMIT $3 OFFSET $4`, readerID, coin, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]Ledger, 0)
	for rows.Next() {
		var v Ledger
		if err := rows.Scan(&v.ID, &v.ReaderID, &v.LedgerNo, &v.BizType, &v.Direction, &v.CoinType, &v.Amount, &v.BalanceBefore, &v.BalanceAfter, &v.BizID, &v.OrderNo, &v.Remark, &v.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, v)
	}
	return items, total, rows.Err()
}

func (r SQLRepository) Adjust(ctx context.Context, in AdjustmentInput) (Adjustment, error) {
	in.ReaderID = strings.TrimSpace(in.ReaderID)
	in.Amount = strings.TrimSpace(in.Amount)
	in.CoinType = strings.TrimSpace(in.CoinType)
	in.Direction = strings.TrimSpace(in.Direction)
	in.Reason = strings.TrimSpace(in.Reason)
	in.RequestID = strings.TrimSpace(in.RequestID)
	readerID, err := strconv.ParseInt(in.ReaderID, 10, 64)
	if err != nil || readerID <= 0 || in.CoinType != "recharge" && in.CoinType != "bonus" || in.Direction != "income" && in.Direction != "expense" || in.RequestID == "" || len(in.RequestID) > 64 || in.Reason == "" || len([]rune(in.Reason)) > 255 {
		return Adjustment{}, ErrInvalidAdjustment
	}
	amount, err := strconv.ParseInt(in.Amount, 10, 64)
	if err != nil || amount <= 0 {
		return Adjustment{}, ErrInvalidAdjustment
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return Adjustment{}, err
	}
	defer tx.Rollback()
	var accountID int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM reader_accounts WHERE id=$1 FOR UPDATE`, readerID).Scan(&accountID); err != nil {
		return Adjustment{}, err
	}
	key := fmt.Sprintf("admin_wallet:%d:%s", readerID, in.RequestID)
	ledgerNo := fmt.Sprintf("AW-%d-%d", readerID, time.Now().UnixNano())
	ledger, err := wallet.MutateTx(ctx, tx, wallet.Mutation{ReaderID: readerID, Amount: amount, CoinType: in.CoinType, Direction: in.Direction, LedgerNo: ledgerNo, BizType: "wallet_adjustment", BizID: &key, Remark: &in.Reason, IdempotencyKey: &key})
	if err != nil {
		return Adjustment{}, err
	}
	if err = tx.Commit(); err != nil {
		return Adjustment{}, err
	}
	return Adjustment{ReaderID: strconv.FormatInt(accountID, 10), RequestID: in.RequestID, Ledger: fromWalletLedger(ledger)}, nil
}

func fromWalletLedger(v wallet.Ledger) Ledger {
	return Ledger{ID: strconv.FormatInt(v.ID, 10), ReaderID: strconv.FormatInt(v.ReaderID, 10), Amount: strconv.FormatInt(v.Amount, 10), BalanceBefore: strconv.FormatInt(v.BalanceBefore, 10), BalanceAfter: strconv.FormatInt(v.BalanceAfter, 10), LedgerNo: v.LedgerNo, BizType: v.BizType, Direction: v.Direction, CoinType: v.CoinType, BizID: valueOrEmpty(v.BizID), Remark: valueOrEmpty(v.Remark), CreatedAt: v.CreatedAt.Format(time.RFC3339Nano)}
}

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
