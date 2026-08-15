package contract

import "errors"

var (
	ErrAccountNotFound        = errors.New("reader account not found")
	ErrAccountDisabled        = errors.New("reader account is disabled")
	ErrAuthenticationInvalid  = errors.New("reader authentication is invalid")
	ErrInviteRelationNotFound = errors.New("reader invite relation not found")
	ErrConflict               = errors.New("reader operation conflicts with current state")
	ErrUnavailable            = errors.New("reader dependency is unavailable")
	ErrTimeout                = errors.New("reader dependency timed out")
	ErrUnknown                = errors.New("reader operation failed")
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
