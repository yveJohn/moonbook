package adminwallet

import "context"

type Wallet struct {
	ReaderID, ReaderUsername                string
	RechargeBalance, BonusBalance           string
	TotalRechargeIncome, TotalBonusIncome   string
	TotalRechargeExpense, TotalBonusExpense string
}
type Ledger struct {
	ID, ReaderID, Amount, BalanceBefore, BalanceAfter string
	LedgerNo, BizType, Direction, CoinType            string
	BizID, OrderNo, Remark                            string
	CreatedAt                                         string
}
type Repository interface {
	ListWallets(context.Context, string, int, int) ([]Wallet, int64, error)
	ListLedgers(context.Context, int64, string, int, int) ([]Ledger, int64, error)
}
