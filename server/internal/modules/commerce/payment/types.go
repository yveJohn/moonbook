package payment

import "context"

type Callback struct {
	PID, TradeID, OrderNo, Amount, ActualAmount, ReceiveAddress, Token, TransactionID, Signature string
	Status                                                                                       int
	Fields                                                                                       map[string]string
}
type Repository interface {
	Process(context.Context, Callback) error
}
