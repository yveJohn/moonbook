package adminwallet

import (
	"context"
	"database/sql"
)

type SQLRepository struct{ DB *sql.DB }

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
