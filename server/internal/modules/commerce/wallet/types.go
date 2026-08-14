package wallet

import (
	"context"
	"time"
)

type Wallet struct {
	ReaderID, RechargeCoinBalance, BonusCoinBalance int64
	TotalRechargeCoinIncome, TotalBonusCoinIncome   int64
	TotalRechargeCoinExpense, TotalBonusCoinExpense int64
}

type Ledger struct {
	ID, ReaderID, Amount, BalanceBefore, BalanceAfter int64
	LedgerNo, BizType, Direction, CoinType            string
	BizID, OrderNo, Remark, IdempotencyKey            *string
	CreatedAt                                         time.Time
}

type Mutation struct {
	ReaderID                               int64
	BizType, Direction, CoinType, LedgerNo string
	BizID, OrderNo, Remark, IdempotencyKey *string
	Amount                                 int64
}

type Repository interface {
	Get(context.Context, int64) (Wallet, error)
	List(context.Context, int64, string, int, int) ([]Ledger, int64, error)
	Mutate(context.Context, Mutation) (Ledger, error)
}
