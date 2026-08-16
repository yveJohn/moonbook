package recharge

import (
	"context"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/epusdt"
)

type Product struct {
	ID, DiamondAmount                  int64
	ProductName, PriceUSDT, SaleStatus string
	SortOrder                          int
}
type Catalog struct {
	Products                                            []Product
	CustomEnabled                                       bool
	DiamondsPerUSDT, MinDiamondAmount, MaxDiamondAmount string
}
type Quote struct{ DiamondAmount, PriceUSDT string }
type Order struct {
	ID, ReaderID, DiamondAmount, ProductID                                       string
	OrderNo, SourceType, PriceUSDT, Provider, Currency, Token, Network           string
	GatewayTradeID, ActualAmount, ReceiveAddress, PaymentURL, BlockTransactionID *string
	Status                                                                       string
	GatewayStatus                                                                *int
	WalletLedgerID, FailureCode, FailureMessage                                  *string
	ExpireTime, PaidTime                                                         *time.Time
	CreateTime, UpdateTime                                                       time.Time
}
type CreateRequest struct {
	ReaderID                       int64
	ProductID                      *int64
	CustomDiamondAmount, RequestID string
}
type PreparedOrder struct {
	ReaderID            int64
	RequestID           string
	SourceType          string
	ProductID           *int64
	DiamondAmount       int64
	PriceUSDT           string
	Provider            string
	Currency            string
	Token               string
	Network             string
	CredentialRef       string
	MerchantPIDSnapshot string
}
type StartResult struct {
	Order   Order
	Created bool
}
type Repository interface {
	Catalog(context.Context) (Catalog, error)
	Quote(context.Context, int64) (Quote, error)
	Prepare(context.Context, CreateRequest) (PreparedOrder, error)
	Start(context.Context, PreparedOrder) (StartResult, error)
	Complete(context.Context, string, epusdt.CreateResponse) (Order, error)
	Fail(context.Context, string, *epusdt.GatewayError) (Order, error)
	GetOrder(context.Context, int64, string) (Order, error)
}
