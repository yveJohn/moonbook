package contract

import (
	"context"
	"time"
)

type AccessRequest struct {
	ReaderID         *int64
	BookID           int64
	ChapterID        int64
	ChargeMode       string
	ChapterWordCount int
	FixedPriceCoin   *int64
}

type AccessResult struct {
	BookID             int64
	ChapterID          *int64
	ChargeMode         string
	Readable           bool
	AccessReason       string
	MembershipEntitled bool
	BookPurchased      bool
	ChapterPurchased   bool
	Purchasable        bool
	ChapterWordCount   int
	PricingWordUnit    int
	PricingCoinUnit    int64
	ChapterPrice       int64
	ProductID          *int64
	ProductName        string
	PriceCoin          int64
	SaleStatus         string
}

// AccessReaders returns one result per request in the same order.
type AccessReader interface {
	AccessReaders(context.Context, []AccessRequest) ([]AccessResult, error)
}

type CheckinStatus struct {
	TodayChecked      bool
	ContinuousDays    int
	TodayRewardCoin   int64
	RewardRandom      bool
	RewardText        string
	UnavailableReason string
	CheckinAvailable  bool
}

type CheckinReader interface {
	CheckinStatus(context.Context, int64) (CheckinStatus, error)
	Checkin(context.Context, int64) (CheckinStatus, error)
}

type Wallet struct {
	ReaderID                 int64
	RechargeCoinBalance      int64
	BonusCoinBalance         int64
	TotalRechargeCoinIncome  int64
	TotalBonusCoinIncome     int64
	TotalRechargeCoinExpense int64
	TotalBonusCoinExpense    int64
}

type WalletLedger struct {
	ID            int64
	ReaderID      int64
	Amount        int64
	BalanceBefore int64
	BalanceAfter  int64
	LedgerNo      string
	BizType       string
	Direction     string
	CoinType      string
	BizID         *string
	OrderNo       *string
	Remark        *string
	CreatedAt     time.Time
}

type WalletReader interface {
	Wallet(context.Context, int64) (Wallet, error)
	WalletLedgers(context.Context, int64, string, int, int) ([]WalletLedger, int64, error)
}

type PurchaseOrder struct {
	ID                 int64
	ReaderID           int64
	OrderNo            string
	OrderType          string
	ProductID          *int64
	ProductType        string
	TargetID           *int64
	BookIDSnapshot     *int64
	ProductName        string
	PriceCoin          int64
	ChapterWordCount   *int
	PricingWordUnit    *int
	PricingCoinUnit    *int64
	RechargeCoinAmount int64
	BonusCoinAmount    int64
	Status             string
	IdempotencyKey     string
	OperatorID         *int64
	Remark             *string
	PaidAt             *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type ChapterQuote struct {
	ChapterID int64
	BookID    int64
	WordCount int
	WordUnit  int
	CoinUnit  int64
	PriceCoin int64
}

type ChapterPurchaseResult struct {
	Status string
	Quote  ChapterQuote
	Order  *PurchaseOrder
}

type PurchaseReader interface {
	BuyMembership(context.Context, int64, int64, string) (PurchaseOrder, error)
	BuyChapter(context.Context, int64, int64, int64, string) (ChapterPurchaseResult, error)
	BuyBook(context.Context, int64, int64, int64) (PurchaseOrder, error)
}

type RechargeProduct struct {
	ID            int64
	Name          string
	DiamondAmount int64
	PriceUSDT     string
	SaleStatus    string
	SortOrder     int
}

type RechargeCatalog struct {
	Products         []RechargeProduct
	CustomEnabled    bool
	DiamondsPerUSDT  string
	MinDiamondAmount string
	MaxDiamondAmount string
}

type RechargeQuote struct {
	DiamondAmount int64
	PriceUSDT     string
}

type RechargeCreateRequest struct {
	ReaderID            int64
	ProductID           *int64
	CustomDiamondAmount *int64
	RequestID           string
}

type RechargeOrder struct {
	ID                 int64
	ReaderID           int64
	DiamondAmount      int64
	ProductID          *int64
	OrderNo            string
	SourceType         string
	PriceUSDT          string
	Provider           string
	Currency           string
	Token              string
	Network            string
	GatewayTradeID     *string
	ActualAmount       *string
	ReceiveAddress     *string
	PaymentURL         *string
	BlockTransactionID *string
	Status             string
	GatewayStatus      *int
	WalletLedgerID     *int64
	FailureCode        *string
	FailureMessage     *string
	ExpiresAt          *time.Time
	PaidAt             *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type RechargeReader interface {
	RechargeCatalog(context.Context) (RechargeCatalog, error)
	QuoteRecharge(context.Context, int64) (RechargeQuote, error)
	CreateRechargeOrder(context.Context, RechargeCreateRequest) (RechargeOrder, error)
	RechargeOrder(context.Context, int64, int64) (RechargeOrder, error)
}
