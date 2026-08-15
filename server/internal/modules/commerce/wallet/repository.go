package wallet

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrInsufficientBalance = errors.New("wallet balance is insufficient")

type SQLRepository struct{ DB *sql.DB }

type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (r SQLRepository) Get(ctx context.Context, readerID int64) (Wallet, error) {
	var w Wallet
	err := r.DB.QueryRowContext(ctx, `INSERT INTO reader_wallets(reader_id) VALUES($1) ON CONFLICT(reader_id) DO NOTHING`, readerID).Err()
	if err != nil {
		return w, err
	}
	err = r.DB.QueryRowContext(ctx, `SELECT reader_id,recharge_coin_balance,bonus_coin_balance,total_recharge_coin_income,total_bonus_coin_income,total_recharge_coin_expense,total_bonus_coin_expense FROM reader_wallets WHERE reader_id=$1`, readerID).Scan(&w.ReaderID, &w.RechargeCoinBalance, &w.BonusCoinBalance, &w.TotalRechargeCoinIncome, &w.TotalBonusCoinIncome, &w.TotalRechargeCoinExpense, &w.TotalBonusCoinExpense)
	return w, err
}

func (r SQLRepository) List(ctx context.Context, readerID int64, coinType string, page, size int) ([]Ledger, int64, error) {
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1 AND ($2='' OR coin_type=$2)`, readerID, coinType).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT id,reader_id,ledger_no,biz_type,biz_id,order_no,direction,coin_type,amount,balance_before,balance_after,remark,idempotency_key,created_at FROM reader_wallet_ledgers WHERE reader_id=$1 AND ($2='' OR coin_type=$2) ORDER BY created_at DESC,id DESC LIMIT $3 OFFSET $4`, readerID, coinType, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Ledger, 0)
	for rows.Next() {
		var v Ledger
		if err := rows.Scan(&v.ID, &v.ReaderID, &v.LedgerNo, &v.BizType, &v.BizID, &v.OrderNo, &v.Direction, &v.CoinType, &v.Amount, &v.BalanceBefore, &v.BalanceAfter, &v.Remark, &v.IdempotencyKey, &v.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

func (r SQLRepository) Mutate(ctx context.Context, m Mutation) (Ledger, error) {
	if m.Amount <= 0 || (m.Direction != "income" && m.Direction != "expense") || (m.CoinType != "recharge" && m.CoinType != "bonus") {
		return Ledger{}, errors.New("invalid wallet mutation")
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return Ledger{}, err
	}
	defer tx.Rollback()
	v, err := MutateTx(ctx, tx, m)
	if err != nil {
		return Ledger{}, err
	}
	if err = tx.Commit(); err != nil {
		return Ledger{}, err
	}
	return v, nil
}

// MutateTx applies a wallet mutation inside an existing transaction.
func MutateTx(ctx context.Context, tx DBTX, m Mutation) (Ledger, error) {
	if m.Amount <= 0 || (m.Direction != "income" && m.Direction != "expense") || (m.CoinType != "recharge" && m.CoinType != "bonus") {
		return Ledger{}, errors.New("invalid wallet mutation")
	}
	if m.IdempotencyKey != nil {
		lockKey := fmt.Sprintf("commerce-wallet:%d:%s", m.ReaderID, *m.IdempotencyKey)
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, lockKey); err != nil {
			return Ledger{}, err
		}
		var v Ledger
		err := scanLedger(tx.QueryRowContext(ctx, `SELECT id,reader_id,ledger_no,biz_type,biz_id,order_no,direction,coin_type,amount,balance_before,balance_after,remark,idempotency_key,created_at FROM reader_wallet_ledgers WHERE reader_id=$1 AND idempotency_key=$2`, m.ReaderID, *m.IdempotencyKey), &v)
		if err == nil {
			return v, nil
		}
		if err != sql.ErrNoRows {
			return Ledger{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO reader_wallets(reader_id) VALUES($1) ON CONFLICT(reader_id) DO NOTHING`, m.ReaderID); err != nil {
		return Ledger{}, err
	}
	var before int64
	col := "recharge_coin_balance"
	if m.CoinType == "bonus" {
		col = "bonus_coin_balance"
	}
	if err := tx.QueryRowContext(ctx, `SELECT `+col+` FROM reader_wallets WHERE reader_id=$1 FOR UPDATE`, m.ReaderID).Scan(&before); err != nil {
		return Ledger{}, err
	}
	after := before + m.Amount
	if m.Direction == "expense" {
		after = before - m.Amount
		if after < 0 {
			return Ledger{}, ErrInsufficientBalance
		}
	}
	totalColumn := map[string]string{"recharge": "total_recharge_coin_", "bonus": "total_bonus_coin_"}[m.CoinType] + map[string]string{"income": "income", "expense": "expense"}[m.Direction]
	if _, err := tx.ExecContext(ctx, `UPDATE reader_wallets SET `+col+`=$1, updated_at=now(), `+totalColumn+`=`+totalColumn+` + $2 WHERE reader_id=$3`, after, m.Amount, m.ReaderID); err != nil {
		return Ledger{}, err
	}
	var v Ledger
	err := scanLedger(tx.QueryRowContext(ctx, `INSERT INTO reader_wallet_ledgers(reader_id,ledger_no,biz_type,biz_id,order_no,direction,coin_type,amount,balance_before,balance_after,remark,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id,reader_id,ledger_no,biz_type,biz_id,order_no,direction,coin_type,amount,balance_before,balance_after,remark,idempotency_key,created_at`, m.ReaderID, m.LedgerNo, m.BizType, m.BizID, m.OrderNo, m.Direction, m.CoinType, m.Amount, before, after, m.Remark, m.IdempotencyKey), &v)
	if err != nil {
		return Ledger{}, err
	}
	return v, nil
}

type rowScanner interface{ Scan(...any) error }

func scanLedger(s rowScanner, v *Ledger) error {
	return s.Scan(&v.ID, &v.ReaderID, &v.LedgerNo, &v.BizType, &v.BizID, &v.OrderNo, &v.Direction, &v.CoinType, &v.Amount, &v.BalanceBefore, &v.BalanceAfter, &v.Remark, &v.IdempotencyKey, &v.CreatedAt)
}
