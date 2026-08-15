package adminrechargeorder

import (
	"context"
	"time"
)

type Order struct {
	ID, ReaderID, DiamondAmount, ProductID                                               string
	ReaderUsername, OrderNo, SourceType, PriceUSDT, Provider, Currency, Token, Network   string
	GatewayTradeID, ActualAmount, ReceiveAddress, PaymentURL, BlockTransactionID, Status string
	GatewayStatus                                                                        *int
	CreatedAt, PaidAt                                                                    *time.Time
}

type Repository interface {
	List(context.Context, string, string, int, int) ([]Order, int64, error)
	Get(context.Context, int64) (Order, error)
	ReaderID(context.Context, int64) (int64, error)
	ManualPay(context.Context, int64, int64, ManualPayInput) (Order, error)
	Sync(context.Context, int64) (Order, error)
}

type Transactor interface {
	Within(context.Context, func(context.Context) error) error
}

type ManualPayInput struct {
	RequestID      string
	GatewayTradeID string
	ActualAmount   string
	Remark         string
}

type CallbackRepository interface {
	ListCallbacks(context.Context, string, string, int, int) ([]CallbackLog, int64, error)
	GetCallback(context.Context, int64) (CallbackLog, error)
}
