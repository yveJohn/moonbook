package payment

import (
	"context"
	"errors"
)

type Callback struct {
	PID, TradeID, OrderNo, Amount, ActualAmount, ReceiveAddress, Token, TransactionID, Signature string
	Status                                                                                       int
	Fields                                                                                       map[string]string
}

type VerificationSnapshot struct {
	OrderID       int64
	CredentialRef string
	MerchantPID   string
}

type Repository interface {
	VerificationSnapshot(context.Context, string) (VerificationSnapshot, error)
	Process(context.Context, Callback) error
	ProcessAttempt(context.Context, int64, Callback) (AttemptResult, error)
}

type CallbackAuditRepository interface {
	BeginAttempt(context.Context, AttemptStart) (int64, error)
	FinalizeAttempt(context.Context, int64, AttemptCompletion) error
}

type AttemptObserver interface {
	ObserveCallbackAttempt(string)
	ObserveCallbackResponse(int)
}

type ProcessingError struct {
	Code           FailureCode
	kind           error
	signatureValid bool
	orderID        int64
}

func (e *ProcessingError) Error() string {
	if reason, ok := FailureReason(e.Code); ok {
		return reason
	}
	return "Callback processing failed"
}

func (e *ProcessingError) Unwrap() error { return e.kind }

func newProcessingError(code FailureCode, kind error) error {
	return &ProcessingError{Code: code, kind: kind}
}

func FailureCodeOf(err error) FailureCode {
	var classified *ProcessingError
	if errors.As(err, &classified) {
		return classified.Code
	}
	return ""
}

func processingErrorDetails(err error) (FailureCode, bool, *int64) {
	var classified *ProcessingError
	if !errors.As(err, &classified) {
		return "", false, nil
	}
	if classified.orderID <= 0 {
		return classified.Code, classified.signatureValid, nil
	}
	orderID := classified.orderID
	return classified.Code, classified.signatureValid, &orderID
}

func enrichProcessingError(err error, signatureValid bool, orderID int64) error {
	var classified *ProcessingError
	if !errors.As(err, &classified) {
		return err
	}
	copy := *classified
	copy.signatureValid = signatureValid
	copy.orderID = orderID
	return &copy
}
