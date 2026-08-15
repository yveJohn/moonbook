package provider

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/wallet"
)

type Wallet struct{ service *wallet.Service }

var _ contract.WalletReader = (*Wallet)(nil)

func NewWallet(service *wallet.Service) *Wallet { return &Wallet{service: service} }

func (provider *Wallet) Wallet(ctx context.Context, readerID int64) (contract.Wallet, error) {
	if provider == nil || provider.service == nil {
		return contract.Wallet{}, contract.ErrUnavailable
	}
	value, err := provider.service.Get(ctx, readerID)
	if err != nil {
		return contract.Wallet{}, contract.Wrap(contract.ErrUnavailable, err)
	}
	return contract.Wallet{
		ReaderID: value.ReaderID, RechargeCoinBalance: value.RechargeCoinBalance,
		BonusCoinBalance: value.BonusCoinBalance, TotalRechargeCoinIncome: value.TotalRechargeCoinIncome,
		TotalBonusCoinIncome: value.TotalBonusCoinIncome, TotalRechargeCoinExpense: value.TotalRechargeCoinExpense,
		TotalBonusCoinExpense: value.TotalBonusCoinExpense,
	}, nil
}

func (provider *Wallet) WalletLedgers(ctx context.Context, readerID int64, coinType string, page, size int) ([]contract.WalletLedger, int64, error) {
	if provider == nil || provider.service == nil {
		return nil, 0, contract.ErrUnavailable
	}
	rows, total, err := provider.service.List(ctx, readerID, coinType, page, size)
	if err != nil {
		return nil, 0, contract.Wrap(contract.ErrUnavailable, err)
	}
	out := make([]contract.WalletLedger, 0, len(rows))
	for _, value := range rows {
		out = append(out, contract.WalletLedger{
			ID: value.ID, ReaderID: value.ReaderID, Amount: value.Amount,
			BalanceBefore: value.BalanceBefore, BalanceAfter: value.BalanceAfter,
			LedgerNo: value.LedgerNo, BizType: value.BizType, Direction: value.Direction,
			CoinType: value.CoinType, BizID: value.BizID, OrderNo: value.OrderNo,
			Remark: value.Remark, CreatedAt: value.CreatedAt,
		})
	}
	return out, total, nil
}
