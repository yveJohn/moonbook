package transaction

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Transactor struct {
	db *sql.DB
}

func New(db *sql.DB) *Transactor {
	return &Transactor{db: db}
}

func (transactor *Transactor) Within(ctx context.Context, fn func(context.Context) error) error {
	if fn == nil {
		return errors.New("transaction callback is required")
	}
	if state, ok := stateFrom(ctx); ok {
		return runNested(ctx, state, fn)
	}
	if transactor == nil || transactor.db == nil {
		return ErrNoDatabase
	}
	tx, err := transactor.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	state := &scopeState{tx: tx}
	txCtx := context.WithValue(ctx, contextKey{}, state)

	defer func() {
		if recovered := recover(); recovered != nil {
			state.markRollback(fmt.Errorf("transaction callback panic: %v", recovered))
			state.finish()
			_ = tx.Rollback()
			panic(recovered)
		}
	}()

	callbackErr := fn(txCtx)
	if callbackErr != nil {
		state.markRollback(callbackErr)
	}
	rollbackOnly, rollbackCause := state.rollbackStatus()
	if rollbackOnly {
		state.finish()
		rollbackErr := tx.Rollback()
		if callbackErr != nil {
			return errors.Join(callbackErr, rollbackErr)
		}
		return errors.Join(ErrRollbackOnly, rollbackCause, rollbackErr)
	}
	if err := tx.Commit(); err != nil {
		state.finish()
		return fmt.Errorf("commit transaction: %w", err)
	}
	callbacks := state.finish()
	return runAfterCommit(context.WithoutCancel(ctx), callbacks)
}

func runNested(ctx context.Context, state *scopeState, fn func(context.Context) error) (err error) {
	if state.isComplete() {
		return ErrTransactionComplete
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			state.markRollback(fmt.Errorf("nested transaction callback panic: %v", recovered))
			panic(recovered)
		}
	}()
	err = fn(ctx)
	if err != nil {
		state.markRollback(err)
	}
	return err
}
