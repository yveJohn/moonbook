package purchase

import (
	"context"
	"time"
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
	BuyMembership(context.Context, int64, string, string) (Order, error)
	BuyChapter(context.Context, int64, string, string, string) (ChapterResult, error)
	BuyBook(context.Context, int64, string, string) (Order, error)
}
