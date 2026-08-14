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
}
