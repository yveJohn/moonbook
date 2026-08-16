package epusdt

import (
	"fmt"
	"time"
)

type FailureClass string

const (
	FailureDefinite  FailureClass = "definite"
	FailureUncertain FailureClass = "uncertain"
)

type GatewayError struct {
	Code    string
	Class   FailureClass
	Summary string
}

func (failure *GatewayError) Error() string {
	if failure == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", failure.Code, failure.Summary)
}

func (failure *GatewayError) Uncertain() bool {
	return failure != nil && failure.Class == FailureUncertain
}

type CreateRequest struct {
	OrderID string
	Amount  string
}

type CreateResponse struct {
	TradeID        string
	OrderID        string
	Amount         string
	Currency       string
	ActualAmount   string
	ReceiveAddress string
	Token          string
	Status         int
	ExpirationTime time.Time
	PaymentURL     string
}
