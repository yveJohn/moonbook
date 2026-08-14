package purchase

import "context"

type Order struct{ ID, ReaderID, OrderNo, OrderType, ProductID, ProductType, TargetID, BookIDSnapshot, ProductName, PriceCoin, ChapterWordCount, PricingWordUnit, PricingCoinUnit, RechargeCoinAmount, BonusCoinAmount, Status, IdempotencyKey string }
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
