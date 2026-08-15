package provider

import (
	"context"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/catalog"
	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
)

type Access struct{ service *catalog.Service }

var _ commercecontract.AccessReader = (*Access)(nil)

func NewAccess(service *catalog.Service) *Access { return &Access{service: service} }

func (provider *Access) AccessReaders(ctx context.Context, requests []commercecontract.AccessRequest) ([]commercecontract.AccessResult, error) {
	if provider == nil || provider.service == nil {
		return nil, commercecontract.ErrUnavailable
	}
	inputs := make([]catalog.AccessRequest, 0, len(requests))
	for _, request := range requests {
		inputs = append(inputs, catalog.AccessRequest{ReaderID: request.ReaderID, BookID: request.BookID, ChapterID: request.ChapterID, ChargeMode: request.ChargeMode, ChapterWordCount: request.ChapterWordCount, FixedPriceCoin: request.FixedPriceCoin})
	}
	results, err := provider.service.AccessReaders(ctx, inputs)
	if err != nil {
		return nil, commercecontract.Wrap(commercecontract.ErrUnavailable, err)
	}
	output := make([]commercecontract.AccessResult, 0, len(results))
	for _, result := range results {
		output = append(output, commercecontract.AccessResult{
			BookID: mustInt64(result.BookID), ChapterID: optionalInt64(result.ChapterID), ChargeMode: result.ChargeMode,
			Readable: result.Readable, AccessReason: result.AccessReason, MembershipEntitled: result.MembershipEntitled,
			BookPurchased: result.BookPurchased, ChapterPurchased: result.ChapterPurchased, Purchasable: result.Purchasable,
			ChapterWordCount: result.ChapterWordCount, PricingWordUnit: result.PricingWordUnit,
			PricingCoinUnit: mustInt64(result.PricingCoinUnit), ChapterPrice: mustInt64(result.ChapterPrice),
			ProductID: optionalInt64(result.ProductID), ProductName: result.ProductName,
			PriceCoin: mustInt64(result.PriceCoin), SaleStatus: result.SaleStatus,
		})
	}
	return output, nil
}
