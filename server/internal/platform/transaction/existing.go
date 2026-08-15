package transaction

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
	PrepareContext(context.Context, string) (*sql.Stmt, error)
}

func Executor(ctx context.Context, fallback *sql.DB) DBTX {
	if state, ok := stateFrom(ctx); ok {
		return state.tx
	}
	return fallback
}

func WithExisting(ctx context.Context, tx *sql.Tx, fn func(context.Context) error) (err error) {
	if tx == nil {
		return ErrNoTransaction
	}
	if fn == nil {
		return errors.New("transaction callback is required")
	}
	if state, ok := stateFrom(ctx); ok {
		if state.tx != tx {
			return ErrTransactionMismatch
		}
		return runNested(ctx, state, fn)
	}
	state := &scopeState{tx: tx, external: true}
	txCtx := context.WithValue(ctx, contextKey{}, state)
	defer func() {
		if recovered := recover(); recovered != nil {
			state.markRollback(fmt.Errorf("external transaction callback panic: %v", recovered))
			state.finish()
			panic(recovered)
		}
	}()
	err = fn(txCtx)
	if err != nil {
		state.markRollback(err)
	}
	rollbackOnly, rollbackCause := state.rollbackStatus()
	state.finish()
	if err != nil {
		return err
	}
	if rollbackOnly {
		return errors.Join(ErrRollbackOnly, rollbackCause)
	}
	return nil
}
