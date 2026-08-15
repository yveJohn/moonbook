package transaction

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrAfterCommit         = errors.New("transaction committed but an after-commit callback failed")
	ErrExternalAfterCommit = errors.New("after-commit callbacks require a transaction owned by Transactor")
)

type Callback func(context.Context) error

type AfterCommitError struct {
	Err error
}

func (err *AfterCommitError) Error() string {
	return fmt.Sprintf("%v: %v", ErrAfterCommit, err.Err)
}

func (err *AfterCommitError) Unwrap() error {
	return errors.Join(ErrAfterCommit, err.Err)
}

func AfterCommit(ctx context.Context, callback Callback) error {
	if callback == nil {
		return errors.New("after-commit callback is required")
	}
	state, ok := stateFrom(ctx)
	if !ok {
		return ErrNoTransaction
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.complete {
		return ErrTransactionComplete
	}
	if state.external {
		return ErrExternalAfterCommit
	}
	state.afterCommit = append(state.afterCommit, callback)
	return nil
}

func runAfterCommit(ctx context.Context, callbacks []Callback) error {
	var callbackErr error
	for _, callback := range callbacks {
		if err := callback(ctx); err != nil {
			callbackErr = errors.Join(callbackErr, err)
		}
	}
	if callbackErr == nil {
		return nil
	}
	return &AfterCommitError{Err: callbackErr}
}
