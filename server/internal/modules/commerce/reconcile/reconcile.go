package reconcile

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
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

// DomainMismatch is deliberately string-shaped so the command output remains
// lossless for migrated bigint IDs and order numbers.
type DomainMismatch struct {
	Domain, Key, Field, Expected, Actual string
}

type FullReport struct {
	Wallets          Report
	RechargeOrders   int
	PurchaseOrders   int
	Callbacks        int
	MembershipGrants int
	Entitlements     int
	Mismatches       []DomainMismatch
}

func mismatch(domain, key, field, expected, actual string) DomainMismatch {
	return DomainMismatch{Domain: domain, Key: key, Field: field, Expected: expected, Actual: actual}
}

// Full checks cross-table financial and entitlement invariants. It is read
// only and can therefore run against a restored production-data copy.
func Full(ctx context.Context, db *sql.DB) (FullReport, error) {
	wallets, err := Wallets(ctx, db)
	if err != nil {
		return FullReport{}, err
	}
	r := FullReport{Wallets: wallets, Mismatches: append([]DomainMismatch{}, walletsToDomain(wallets)...)}

	rows, err := db.QueryContext(ctx, `SELECT id::text,reader_id::text,order_no,diamond_amount::text,COALESCE(wallet_ledger_id::text,'') FROM reader_recharge_orders WHERE status='paid' OR wallet_ledger_id IS NOT NULL ORDER BY id`)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var id, readerID, orderNo, amount, ledgerID string
		if err := rows.Scan(&id, &readerID, &orderNo, &amount, &ledgerID); err != nil {
			rows.Close()
			return r, err
		}
		r.RechargeOrders++
		if ledgerID == "" {
			r.Mismatches = append(r.Mismatches, mismatch("recharge_order", id, "wallet_ledger_id", "non-empty", ""))
			continue
		}
		var ledgerReader, ledgerAmount, direction, coinType, ledgerOrder string
		qerr := db.QueryRowContext(ctx, `SELECT reader_id::text,amount::text,direction,coin_type,COALESCE(order_no,'') FROM reader_wallet_ledgers WHERE id=$1`, ledgerID).Scan(&ledgerReader, &ledgerAmount, &direction, &coinType, &ledgerOrder)
		if qerr == sql.ErrNoRows {
			r.Mismatches = append(r.Mismatches, mismatch("recharge_order", id, "ledger", "existing", "missing"))
			continue
		}
		if qerr != nil {
			rows.Close()
			return r, qerr
		}
		if ledgerReader != readerID {
			r.Mismatches = append(r.Mismatches, mismatch("recharge_order", id, "ledger_reader_id", readerID, ledgerReader))
		}
		if ledgerAmount != amount {
			r.Mismatches = append(r.Mismatches, mismatch("recharge_order", id, "ledger_amount", amount, ledgerAmount))
		}
		if direction != "income" || coinType != "recharge" {
			r.Mismatches = append(r.Mismatches, mismatch("recharge_order", id, "ledger_kind", "income/recharge", direction+"/"+coinType))
		}
		if ledgerOrder != "" && ledgerOrder != orderNo {
			r.Mismatches = append(r.Mismatches, mismatch("recharge_order", id, "ledger_order_no", orderNo, ledgerOrder))
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return r, err
	}
	for _, q := range []struct{ field, query string }{
		{"gateway_trade_id", `SELECT gateway_trade_id,count(*) FROM reader_recharge_orders WHERE status='paid' AND gateway_trade_id IS NOT NULL AND gateway_trade_id<>'' GROUP BY gateway_trade_id HAVING count(*)>1`},
		{"block_transaction_id", `SELECT block_transaction_id,count(*) FROM reader_recharge_orders WHERE status='paid' AND block_transaction_id IS NOT NULL AND block_transaction_id<>'' GROUP BY block_transaction_id HAVING count(*)>1`},
	} {
		dups, qerr := db.QueryContext(ctx, q.query)
		if qerr != nil {
			return r, qerr
		}
		for dups.Next() {
			var key string
			var count int64
			if err := dups.Scan(&key, &count); err != nil {
				dups.Close()
				return r, err
			}
			r.Mismatches = append(r.Mismatches, mismatch("recharge_order", key, q.field+"_unique", "1", formatAmount(count)))
		}
		if err := dups.Err(); err != nil {
			dups.Close()
			return r, err
		}
		dups.Close()
	}

	rows, err = db.QueryContext(ctx, `SELECT id::text,reader_id::text,order_no,order_type,recharge_coin_amount::text,bonus_coin_amount::text,status FROM reader_purchase_orders ORDER BY id`)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var id, readerID, orderNo, orderType, recharge, bonus, status string
		if err := rows.Scan(&id, &readerID, &orderNo, &orderType, &recharge, &bonus, &status); err != nil {
			rows.Close()
			return r, err
		}
		r.PurchaseOrders++
		var incomeRecharge, incomeBonus, expenseRecharge, expenseBonus int64
		if err := db.QueryRowContext(ctx, `SELECT COALESCE(sum(amount) FILTER(WHERE direction='income' AND coin_type='recharge'),0),COALESCE(sum(amount) FILTER(WHERE direction='income' AND coin_type='bonus'),0),COALESCE(sum(amount) FILTER(WHERE direction='expense' AND coin_type='recharge'),0),COALESCE(sum(amount) FILTER(WHERE direction='expense' AND coin_type='bonus'),0) FROM reader_wallet_ledgers WHERE reader_id=$1 AND order_no=$2`, readerID, orderNo).Scan(&incomeRecharge, &incomeBonus, &expenseRecharge, &expenseBonus); err != nil {
			rows.Close()
			return r, err
		}
		if status == "paid" {
			if orderType == "mock_recharge" {
				if expected, ok := parseAmount(recharge); ok && incomeRecharge != expected {
					r.Mismatches = append(r.Mismatches, mismatch("purchase_order", id, "mock_recharge_income", recharge, formatAmount(incomeRecharge)))
				}
				if expenseRecharge != 0 || incomeBonus != 0 || expenseBonus != 0 {
					r.Mismatches = append(r.Mismatches, mismatch("purchase_order", id, "mock_recharge_extra_ledger", "0", formatAmount(expenseRecharge+incomeBonus+expenseBonus)))
				}
				continue
			}
			if expected, ok := parseAmount(recharge); ok && expenseRecharge != expected {
				r.Mismatches = append(r.Mismatches, mismatch("purchase_order", id, "recharge_expense", recharge, formatAmount(expenseRecharge)))
			}
			if expected, ok := parseAmount(bonus); ok && expenseBonus != expected {
				r.Mismatches = append(r.Mismatches, mismatch("purchase_order", id, "bonus_expense", bonus, formatAmount(expenseBonus)))
			}
			if incomeRecharge != 0 || incomeBonus != 0 {
				r.Mismatches = append(r.Mismatches, mismatch("purchase_order", id, "income_ledger", "0", formatAmount(incomeRecharge+incomeBonus)))
			}
			var rights int
			if orderType == "membership" {
				if err := db.QueryRowContext(ctx, `SELECT count(*) FROM commerce_membership_grants WHERE source_type='order' AND source_ref=$1 AND status='active'`, orderNo).Scan(&rights); err != nil {
					rows.Close()
					return r, err
				}
				if rights != 1 {
					r.Mismatches = append(r.Mismatches, mismatch("purchase_order", id, "active_membership_grants", "1", formatAmount(int64(rights))))
				}
			} else if orderType == "chapter" || orderType == "book" {
				if err := db.QueryRowContext(ctx, `SELECT count(*) FROM commerce_entitlements WHERE source_type='order' AND source_ref=$1 AND status='active'`, orderNo).Scan(&rights); err != nil {
					rows.Close()
					return r, err
				}
				if rights != 1 {
					r.Mismatches = append(r.Mismatches, mismatch("purchase_order", id, "active_entitlements", "1", formatAmount(int64(rights))))
				}
			}
		} else {
			var active int
			if err := db.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM commerce_membership_grants WHERE source_type='order' AND source_ref=$1 AND status='active') + (SELECT count(*) FROM commerce_entitlements WHERE source_type='order' AND source_ref=$1 AND status='active')`, orderNo).Scan(&active); err != nil {
				rows.Close()
				return r, err
			}
			if active != 0 {
				r.Mismatches = append(r.Mismatches, mismatch("purchase_order", id, "active_rights", "0", formatAmount(int64(active))))
			}
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return r, err
	}

	rows, err = db.QueryContext(ctx, `SELECT id::text,COALESCE(recharge_order_id::text,''),processing_result FROM reader_payment_callback_logs WHERE processing_result IN ('success','manual_success') ORDER BY id`)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var id, orderID, result string
		if err := rows.Scan(&id, &orderID, &result); err != nil {
			rows.Close()
			return r, err
		}
		r.Callbacks++
		if orderID == "" {
			r.Mismatches = append(r.Mismatches, mismatch("callback", id, "recharge_order_id", "non-empty", ""))
			continue
		}
		var status, ledgerID string
		if err := db.QueryRowContext(ctx, `SELECT status,COALESCE(wallet_ledger_id::text,'') FROM reader_recharge_orders WHERE id=$1`, orderID).Scan(&status, &ledgerID); err == sql.ErrNoRows {
			r.Mismatches = append(r.Mismatches, mismatch("callback", id, "order", "existing paid", "missing"))
			continue
		} else if err != nil {
			rows.Close()
			return r, err
		}
		if status != "paid" || ledgerID == "" {
			r.Mismatches = append(r.Mismatches, mismatch("callback", id, "order_state", "paid with ledger", status+"/"+ledgerID))
		}
	}
	rows.Close()
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM commerce_membership_grants`).Scan(&r.MembershipGrants); err != nil {
		return r, err
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM commerce_entitlements`).Scan(&r.Entitlements); err != nil {
		return r, err
	}
	return r, rows.Err()
}

func walletsToDomain(r Report) []DomainMismatch {
	out := make([]DomainMismatch, 0, len(r.Mismatches))
	for _, m := range r.Mismatches {
		out = append(out, mismatch("wallet", formatAmount(m.ReaderID), m.CoinType+"."+m.Field, formatAmount(m.Expected), formatAmount(m.Actual)))
	}
	return out
}
func parseAmount(v string) (int64, bool) {
	var n int64
	_, err := fmt.Sscan(v, &n)
	return n, err == nil
}
func formatAmount(v int64) string { return strconv.FormatInt(v, 10) }

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
