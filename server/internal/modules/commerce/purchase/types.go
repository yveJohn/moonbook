package purchase

import (
	"context"
	"time"

	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
)

type Order struct {
	ID, ReaderID, OrderNo, OrderType, ProductID, ProductType, TargetID, BookIDSnapshot string
	ProductName, PriceCoin, ChapterWordCount, PricingWordUnit, PricingCoinUnit         string
	RechargeCoinAmount, BonusCoinAmount, Status, IdempotencyKey, OperatorID            string
	Remark                                                                             *string
	PaidTime                                                                           *time.Time
	CreateTime, UpdateTime                                                             time.Time
}
type ChapterQuote struct {
	ChapterID, BookID   string
	WordCount, WordUnit int
	CoinUnit, PriceCoin string
}
type ChapterResult struct {
	PurchaseStatus string
	Quote          ChapterQuote
	Order          *Order
}
type Repository interface {
	LockIdempotency(context.Context, int64) error
	FindOrder(context.Context, int64, string) (Order, error)
	BuyMembership(context.Context, int64, int64, string) (Order, error)
	BuyChapter(context.Context, int64, novelcontract.PurchaseSnapshot, int64, string) (ChapterResult, error)
	BuyBook(context.Context, int64, novelcontract.PurchaseSnapshot, int64, string) (Order, error)
}

type Transactor interface {
	Within(context.Context, func(context.Context) error) error
}
