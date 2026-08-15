package transaction

import (
	"context"
	"database/sql"
	"errors"
	"sync"
)

var (
	ErrNoDatabase          = errors.New("transaction database is not configured")
	ErrNoTransaction       = errors.New("context does not contain a transaction")
	ErrRollbackOnly        = errors.New("transaction is marked rollback-only")
	ErrTransactionComplete = errors.New("transaction scope is already complete")
	ErrTransactionMismatch = errors.New("context contains a different transaction")
)

type contextKey struct{}

type scopeState struct {
	tx       *sql.Tx
	external bool

	mu            sync.Mutex
	complete      bool
	rollbackOnly  bool
	rollbackCause error
	afterCommit   []Callback
}

func stateFrom(ctx context.Context) (*scopeState, bool) {
	state, ok := ctx.Value(contextKey{}).(*scopeState)
	return state, ok && state != nil
}

func (state *scopeState) markRollback(cause error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.rollbackOnly = true
	if cause != nil {
		state.rollbackCause = errors.Join(state.rollbackCause, cause)
	}
}

func (state *scopeState) rollbackStatus() (bool, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.rollbackOnly, state.rollbackCause
}

func (state *scopeState) isComplete() bool {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.complete
}

func (state *scopeState) finish() []Callback {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.complete = true
	callbacks := append([]Callback(nil), state.afterCommit...)
	state.afterCommit = nil
	return callbacks
}
