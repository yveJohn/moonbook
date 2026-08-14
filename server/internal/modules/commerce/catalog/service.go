package catalog

import (
	"context"
	"errors"
)

var ErrRepositoryUnavailable = errors.New("commerce catalog repository unavailable")

type Service struct {
	Repo  Repository
	Batch BatchRepository
}

func NewService(repo Repository) *Service {
	s := &Service{Repo: repo}
	if b, ok := repo.(BatchRepository); ok {
		s.Batch = b
	}
	return s
}

func (s *Service) AccessReader(ctx context.Context, req AccessRequest) (AccessResult, error) {
	if s == nil || s.Repo == nil {
		return AccessResult{}, ErrRepositoryUnavailable
	}
	c, err := s.Repo.LoadAccessContext(ctx, req)
	if err != nil {
		return AccessResult{}, err
	}
	return decide(req, c), nil
}

func (s *Service) AccessReaders(ctx context.Context, reqs []AccessRequest) ([]AccessResult, error) {
	if s == nil || s.Repo == nil {
		return nil, ErrRepositoryUnavailable
	}
	contexts := make(map[string]AccessContext, len(reqs))
	var err error
	if s.Batch != nil {
		contexts, err = s.Batch.LoadAccessContexts(ctx, reqs)
	} else {
		for _, r := range reqs {
			contexts[key(r)], err = s.Repo.LoadAccessContext(ctx, r)
			if err != nil {
				break
			}
		}
	}
	if err != nil {
		return nil, err
	}
	out := make([]AccessResult, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, decide(r, contexts[key(r)]))
	}
	return out, nil
}

// BatchAccessReader is the explicit contract name used by reader callers.
// AccessReaders remains as a concise Go-friendly alias.
func (s *Service) BatchAccessReader(ctx context.Context, reqs []AccessRequest) ([]AccessResult, error) {
	return s.AccessReaders(ctx, reqs)
}

func decide(req AccessRequest, c AccessContext) AccessResult {
	if c.Reader.ReaderID == nil {
		c.Reader.ReaderID = req.ReaderID
	}
	r := AccessResult{BookID: decimal(req.BookID), ChargeMode: req.ChargeMode, ChapterWordCount: req.ChapterWordCount, PricingWordUnit: c.Pricing.WordUnit, MembershipEntitled: c.Reader.Membership, BookPurchased: c.Reader.BookOwned, ChapterPurchased: c.Reader.ChapterOwned}
	if req.ChapterID != 0 {
		r.ChapterID = decimal(req.ChapterID)
	}
	if c.Product != nil {
		r.ProductID, r.ProductName, r.PriceCoin, r.SaleStatus = decimal(c.Product.ID), c.Product.ProductName, decimal(c.Product.PriceCoin), c.Product.SaleStatus
	}
	if c.Reader.ReaderID == nil {
		r.AccessReason = string(LoginRequired)
		return r
	}
	if c.Reader.BookOwned {
		r.Readable, r.AccessReason = true, string(BookOwned)
		return r
	}
	if c.Reader.ChapterOwned {
		r.Readable, r.AccessReason = true, string(ChapterOwned)
		return r
	}
	if c.Reader.Membership {
		r.Readable, r.AccessReason = true, string(Membership)
		return r
	}
	switch ChargeMode(req.ChargeMode) {
	case LoginFree:
		r.Readable, r.AccessReason = true, string(LoginFreeReason)
	case MembershipOnly:
		r.AccessReason = string(MembershipRequired)
	case FixedPrice:
		r.AccessReason = string(BookPurchaseRequired)
		r.Purchasable = c.Product != nil && c.Product.SaleStatus == "on_sale"
	case WordCharge:
		price := int64(0)
		if c.ChapterProduct != nil {
			price = c.ChapterProduct.PriceCoin
		} else if c.Pricing.Enabled && c.Pricing.WordUnit > 0 && req.ChapterWordCount > 0 {
			price = int64((req.ChapterWordCount+c.Pricing.WordUnit-1)/c.Pricing.WordUnit) * c.Pricing.CoinUnit
		}
		r.ChapterPrice = decimal(price)
		r.PricingCoinUnit = decimal(c.Pricing.CoinUnit)
		if price == 0 {
			r.Readable, r.AccessReason = true, string(FreeChapter)
		} else {
			r.AccessReason = string(ChapterPurchaseRequired)
			r.Purchasable = (c.ChapterProduct != nil && c.ChapterProduct.SaleStatus == "on_sale") || (c.ChapterProduct == nil && c.Pricing.Enabled)
		}
	default:
		r.AccessReason = string(UnsupportedMode)
	}
	return r
}
