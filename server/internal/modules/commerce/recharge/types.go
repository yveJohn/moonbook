package recharge

import (
	"context"
	"time"
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
	WalletLedgerID, ExpireTime, PaidTime, FailureCode, FailureMessage            *string
	CreateTime, UpdateTime                                                       time.Time
}
type CreateRequest struct {
	ReaderID                       int64
	ProductID                      *int64
	CustomDiamondAmount, RequestID string
}
type Repository interface {
	Catalog(context.Context) (Catalog, error)
	Quote(context.Context, int64) (Quote, error)
	CreateOrder(context.Context, CreateRequest) (Order, error)
	GetOrder(context.Context, int64, string) (Order, error)
}
