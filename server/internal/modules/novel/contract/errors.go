package contract

import "errors"

var (
	ErrBookNotFound      = errors.New("novel book not found")
	ErrChapterNotFound   = errors.New("novel chapter not found")
	ErrNotPublished      = errors.New("novel content is not published")
	ErrObjectUnavailable = errors.New("novel object is unavailable")
	ErrObjectIntegrity   = errors.New("novel object integrity check failed")
	ErrTargetUnavailable = errors.New("novel purchase target is unavailable")
	ErrUnavailable       = errors.New("novel dependency is unavailable")
	ErrTimeout           = errors.New("novel dependency timed out")
	ErrUnknown           = errors.New("novel operation failed")
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
