package provider

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/recharge"
)

type Recharge struct{ service *recharge.Service }

var _ contract.RechargeReader = (*Recharge)(nil)

func NewRecharge(service *recharge.Service) *Recharge { return &Recharge{service: service} }

func (provider *Recharge) RechargeCatalog(ctx context.Context) (contract.RechargeCatalog, error) {
	if provider == nil || provider.service == nil {
		return contract.RechargeCatalog{}, contract.ErrUnavailable
	}
	catalog, err := provider.service.Catalog(ctx)
	if err != nil {
		return contract.RechargeCatalog{}, rechargeError(err)
	}
	products := make([]contract.RechargeProduct, 0, len(catalog.Products))
	for _, product := range catalog.Products {
		products = append(products, contract.RechargeProduct{ID: product.ID, Name: product.ProductName, DiamondAmount: product.DiamondAmount, PriceUSDT: product.PriceUSDT, SaleStatus: product.SaleStatus, SortOrder: product.SortOrder})
	}
	return contract.RechargeCatalog{Products: products, CustomEnabled: catalog.CustomEnabled, DiamondsPerUSDT: catalog.DiamondsPerUSDT, MinDiamondAmount: catalog.MinDiamondAmount, MaxDiamondAmount: catalog.MaxDiamondAmount}, nil
}

func (provider *Recharge) QuoteRecharge(ctx context.Context, amount int64) (contract.RechargeQuote, error) {
	if provider == nil || provider.service == nil {
		return contract.RechargeQuote{}, contract.ErrUnavailable
	}
	quote, err := provider.service.Quote(ctx, amount)
	if err != nil {
		return contract.RechargeQuote{}, rechargeError(err)
	}
	return contract.RechargeQuote{DiamondAmount: mustInt64(quote.DiamondAmount), PriceUSDT: quote.PriceUSDT}, nil
}

func (provider *Recharge) CreateRechargeOrder(ctx context.Context, request contract.RechargeCreateRequest) (contract.RechargeOrder, error) {
	if provider == nil || provider.service == nil {
		return contract.RechargeOrder{}, contract.ErrUnavailable
	}
	custom := ""
	if request.CustomDiamondAmount != nil {
		custom = strconv.FormatInt(*request.CustomDiamondAmount, 10)
	}
	order, err := provider.service.CreateOrder(ctx, recharge.CreateRequest{ReaderID: request.ReaderID, ProductID: request.ProductID, CustomDiamondAmount: custom, RequestID: request.RequestID})
	return rechargeOrder(order), rechargeError(err)
}

func (provider *Recharge) RechargeOrder(ctx context.Context, readerID, orderID int64) (contract.RechargeOrder, error) {
	if provider == nil || provider.service == nil {
		return contract.RechargeOrder{}, contract.ErrUnavailable
	}
	order, err := provider.service.GetOrder(ctx, readerID, strconv.FormatInt(orderID, 10))
	return rechargeOrder(order), rechargeError(err)
}

func rechargeOrder(order recharge.Order) contract.RechargeOrder {
	return contract.RechargeOrder{
		ID: mustInt64(order.ID), ReaderID: mustInt64(order.ReaderID), DiamondAmount: mustInt64(order.DiamondAmount),
		ProductID: optionalInt64(order.ProductID), OrderNo: order.OrderNo, SourceType: order.SourceType,
		PriceUSDT: order.PriceUSDT, Provider: order.Provider, Currency: order.Currency, Token: order.Token,
		Network: order.Network, GatewayTradeID: order.GatewayTradeID, ActualAmount: order.ActualAmount,
		ReceiveAddress: order.ReceiveAddress, PaymentURL: order.PaymentURL, BlockTransactionID: order.BlockTransactionID,
		Status: order.Status, GatewayStatus: order.GatewayStatus, WalletLedgerID: optionalStringInt64(order.WalletLedgerID),
		FailureCode: order.FailureCode, FailureMessage: order.FailureMessage, ExpiresAt: order.ExpireTime,
		PaidAt: order.PaidTime, CreatedAt: order.CreateTime, UpdatedAt: order.UpdateTime,
	}
}

func optionalStringInt64(value *string) *int64 {
	if value == nil {
		return nil
	}
	return optionalInt64(*value)
}

func rechargeError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, recharge.ErrInvalidRecharge):
		return contract.Wrap(contract.ErrInvalidRequest, err)
	case errors.Is(err, recharge.ErrRechargeProductUnavailable):
		return contract.Wrap(contract.ErrProductUnavailable, err)
	case errors.Is(err, sql.ErrNoRows):
		return contract.Wrap(contract.ErrNotFound, err)
	default:
		return contract.Wrap(contract.ErrUnavailable, err)
	}
}
