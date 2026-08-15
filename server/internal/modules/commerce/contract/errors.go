package contract

import "errors"

var (
	ErrProductUnavailable  = errors.New("commerce product is unavailable")
	ErrQuoteChanged        = errors.New("commerce quote changed")
	ErrInsufficientFunds   = errors.New("commerce wallet balance is insufficient")
	ErrIdempotencyConflict = errors.New("commerce idempotency conflict")
	ErrPaymentConflict     = errors.New("commerce payment state conflict")
	ErrInvalidRequest      = errors.New("commerce request is invalid")
	ErrNotFound            = errors.New("commerce fact not found")
	ErrUnavailable         = errors.New("commerce dependency is unavailable")
	ErrTimeout             = errors.New("commerce dependency timed out")
	ErrUnknown             = errors.New("commerce operation failed")
)

type classifiedError struct {
	kind  error
	cause error
}

func (err *classifiedError) Error() string { return err.kind.Error() }
func (err *classifiedError) Unwrap() error { return err.kind }

func Wrap(kind, cause error) error {
	if kind == nil {
		kind = ErrUnknown
	}
	if cause == nil {
		return kind
	}
	return &classifiedError{kind: kind, cause: cause}
}

func Cause(err error) error {
	var classified *classifiedError
	if errors.As(err, &classified) {
		return classified.cause
	}
	return nil
}
