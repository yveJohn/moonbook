package provider

import (
	"context"
	"errors"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/purchase"
)

type Purchase struct{ service *purchase.Service }

var _ contract.PurchaseReader = (*Purchase)(nil)

func NewPurchase(service *purchase.Service) *Purchase { return &Purchase{service: service} }

func (provider *Purchase) BuyMembership(ctx context.Context, readerID, productID int64, requestID string) (contract.PurchaseOrder, error) {
	if provider == nil || provider.service == nil {
		return contract.PurchaseOrder{}, contract.ErrUnavailable
	}
	order, err := provider.service.BuyMembership(ctx, readerID, strconv.FormatInt(productID, 10), requestID)
	return purchaseOrder(order), purchaseError(err)
}

func (provider *Purchase) BuyChapter(ctx context.Context, readerID, chapterID, expectedPrice int64, requestID string) (contract.ChapterPurchaseResult, error) {
	if provider == nil || provider.service == nil {
		return contract.ChapterPurchaseResult{}, contract.ErrUnavailable
	}
	result, err := provider.service.BuyChapter(ctx, readerID, strconv.FormatInt(chapterID, 10), strconv.FormatInt(expectedPrice, 10), requestID)
	if err != nil {
		return contract.ChapterPurchaseResult{}, purchaseError(err)
	}
	out := contract.ChapterPurchaseResult{Status: result.PurchaseStatus, Quote: contract.ChapterQuote{
		ChapterID: mustInt64(result.Quote.ChapterID), BookID: mustInt64(result.Quote.BookID),
		WordCount: result.Quote.WordCount, WordUnit: result.Quote.WordUnit,
		CoinUnit: mustInt64(result.Quote.CoinUnit), PriceCoin: mustInt64(result.Quote.PriceCoin),
	}}
	if result.Order != nil {
		order := purchaseOrder(*result.Order)
		out.Order = &order
	}
	return out, nil
}

func (provider *Purchase) BuyBook(ctx context.Context, readerID, bookID, expectedPrice int64) (contract.PurchaseOrder, error) {
	if provider == nil || provider.service == nil {
		return contract.PurchaseOrder{}, contract.ErrUnavailable
	}
	order, err := provider.service.BuyBook(ctx, readerID, strconv.FormatInt(bookID, 10), strconv.FormatInt(expectedPrice, 10))
	return purchaseOrder(order), purchaseError(err)
}

func purchaseOrder(order purchase.Order) contract.PurchaseOrder {
	return contract.PurchaseOrder{
		ID: mustInt64(order.ID), ReaderID: mustInt64(order.ReaderID), OrderNo: order.OrderNo,
		OrderType: order.OrderType, ProductID: optionalInt64(order.ProductID), ProductType: order.ProductType,
		TargetID: optionalInt64(order.TargetID), BookIDSnapshot: optionalInt64(order.BookIDSnapshot),
		ProductName: order.ProductName, PriceCoin: mustInt64(order.PriceCoin),
		ChapterWordCount: optionalInt(order.ChapterWordCount), PricingWordUnit: optionalInt(order.PricingWordUnit),
		PricingCoinUnit: optionalInt64(order.PricingCoinUnit), RechargeCoinAmount: mustInt64(order.RechargeCoinAmount),
		BonusCoinAmount: mustInt64(order.BonusCoinAmount), Status: order.Status, IdempotencyKey: order.IdempotencyKey,
		OperatorID: optionalInt64(order.OperatorID), Remark: order.Remark, PaidAt: order.PaidTime,
		CreatedAt: order.CreateTime, UpdatedAt: order.UpdateTime,
	}
}

func purchaseError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, purchase.ErrInvalidRequest):
		return contract.Wrap(contract.ErrInvalidRequest, err)
	case errors.Is(err, purchase.ErrQuoteChanged):
		return contract.Wrap(contract.ErrQuoteChanged, err)
	case errors.Is(err, purchase.ErrProductUnavailable):
		return contract.Wrap(contract.ErrProductUnavailable, err)
	case errors.Is(err, purchase.ErrInsufficientBalance):
		return contract.Wrap(contract.ErrInsufficientFunds, err)
	default:
		return contract.Wrap(contract.ErrUnavailable, err)
	}
}

func mustInt64(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

func optionalInt64(value string) *int64 {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed == 0 {
		return nil
	}
	return &parsed
}

func optionalInt(value string) *int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed == 0 {
		return nil
	}
	return &parsed
}
