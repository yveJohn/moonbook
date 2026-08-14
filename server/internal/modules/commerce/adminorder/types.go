package adminorder

import (
	"context"
	"time"
)

type Order struct {
	ID, ReaderID, ReaderUsername, OrderNo, OrderType, ProductID, ProductType, TargetID, BookIDSnapshot string
	ProductName, PriceCoin, ChapterWordCount, PricingWordUnit, PricingCoinUnit                         string
	RechargeCoinAmount, BonusCoinAmount, Status, IdempotencyKey, Remark                                string
	PaidAt, CreatedAt, UpdatedAt                                                                       *time.Time
}

type MockRechargeInput struct {
	ReaderID, RechargeCoinAmount, RequestID, Remark string
}

type Repository interface {
	List(context.Context, string, string, string, int, int) ([]Order, int64, error)
	Get(context.Context, int64) (Order, error)
	CreateMockRecharge(context.Context, MockRechargeInput, int64) (Order, error)
	ConfirmMockRecharge(context.Context, int64, int64) (Order, error)
}
