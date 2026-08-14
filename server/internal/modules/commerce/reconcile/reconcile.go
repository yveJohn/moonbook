package reconcile

import (
	"context"
	"database/sql"
)

type Mismatch struct {
	ReaderID         int64
	CoinType, Field  string
	Expected, Actual int64
}
type Report struct {
	Checked    int
	Mismatches []Mismatch
}

func Wallets(ctx context.Context, db *sql.DB) (Report, error) {
	rows, err := db.QueryContext(ctx, `SELECT reader_id,recharge_coin_balance,bonus_coin_balance,total_recharge_coin_income,total_bonus_coin_income,total_recharge_coin_expense,total_bonus_coin_expense FROM reader_wallets ORDER BY reader_id`)
	if err != nil {
		return Report{}, err
	}
	defer rows.Close()
	report := Report{Mismatches: []Mismatch{}}
	for rows.Next() {
		var rid, rb, bb, ri, bi, re, be int64
		if err := rows.Scan(&rid, &rb, &bb, &ri, &bi, &re, &be); err != nil {
			return report, err
		}
		report.Checked++
		for _, c := range []struct {
			typ                      string
			balance, income, expense int64
		}{{"recharge", rb, ri, re}, {"bonus", bb, bi, be}} {
			var ei, ee int64
			if err := db.QueryRowContext(ctx, `SELECT COALESCE(sum(amount) FILTER(WHERE direction='income'),0),COALESCE(sum(amount) FILTER(WHERE direction='expense'),0) FROM reader_wallet_ledgers WHERE reader_id=$1 AND coin_type=$2`, rid, c.typ).Scan(&ei, &ee); err != nil {
				return report, err
			}
			for _, m := range []struct {
				field            string
				expected, actual int64
			}{{"balance", ei - ee, c.balance}, {"income", ei, c.income}, {"expense", ee, c.expense}} {
				if m.expected != m.actual {
					report.Mismatches = append(report.Mismatches, Mismatch{ReaderID: rid, CoinType: c.typ, Field: m.field, Expected: m.expected, Actual: m.actual})
				}
			}
		}
	}
	return report, rows.Err()
}
