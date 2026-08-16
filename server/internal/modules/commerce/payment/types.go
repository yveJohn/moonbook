package payment

import "context"

type Callback struct {
	PID, TradeID, OrderNo, Amount, ActualAmount, ReceiveAddress, Token, TransactionID, Signature string
	Status                                                                                       int
	Fields                                                                                       map[string]string
}

type VerificationSnapshot struct {
	CredentialRef string
	MerchantPID   string
}

type Repository interface {
	VerificationSnapshot(context.Context, string) (VerificationSnapshot, error)
	Process(context.Context, Callback) error
}
