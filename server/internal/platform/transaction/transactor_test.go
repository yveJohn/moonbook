package transaction

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
)

func TestWithinCommitsAndRunsAfterCommitInOrder(t *testing.T) {
	db, state := openScriptedDB(t)
	transactor := New(db)
	var calls []string

	err := transactor.Within(context.Background(), func(ctx context.Context) error {
		if Executor(ctx, db) == db {
			t.Fatal("Executor returned the database instead of the active transaction")
		}
		if err := AfterCommit(ctx, func(callbackCtx context.Context) error {
			if Executor(callbackCtx, db) != db {
				t.Fatal("AfterCommit inherited the completed transaction")
			}
			calls = append(calls, "first")
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return AfterCommit(ctx, func(context.Context) error {
			calls = append(calls, "second")
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.commits != 1 || state.rollbacks != 0 {
		t.Fatalf("commits=%d rollbacks=%d", state.commits, state.rollbacks)
	}
	if fmt.Sprint(calls) != "[first second]" {
		t.Fatalf("AfterCommit order=%v", calls)
	}
	if Executor(context.Background(), db) != db {
		t.Fatal("Executor did not return the fallback database outside a transaction")
	}
	if err := AfterCommit(context.Background(), func(context.Context) error { return nil }); !errors.Is(err, ErrNoTransaction) {
		t.Fatalf("AfterCommit outside transaction error=%v", err)
	}
}

func TestWithinRollsBackCallbackErrorAndSkipsAfterCommit(t *testing.T) {
	db, state := openScriptedDB(t)
	transactor := New(db)
	want := errors.New("business failure")
	called := false

	err := transactor.Within(context.Background(), func(ctx context.Context) error {
		if err := AfterCommit(ctx, func(context.Context) error {
			called = true
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("Within error=%v", err)
	}
	if state.commits != 0 || state.rollbacks != 1 || called {
		t.Fatalf("commits=%d rollbacks=%d afterCommit=%t", state.commits, state.rollbacks, called)
	}
}

func TestWithinPreservesRollbackFailure(t *testing.T) {
	db, state := openScriptedDB(t)
	businessErr := errors.New("business failure")
	rollbackErr := errors.New("rollback failure")
	state.rollbackErr = rollbackErr

	err := New(db).Within(context.Background(), func(context.Context) error { return businessErr })
	if !errors.Is(err, businessErr) || !errors.Is(err, rollbackErr) {
		t.Fatalf("Within error=%v", err)
	}
}

func TestNestedFailureMarksOuterTransactionRollbackOnly(t *testing.T) {
	db, state := openScriptedDB(t)
	transactor := New(db)
	want := errors.New("nested failure")

	err := transactor.Within(context.Background(), func(ctx context.Context) error {
		nestedErr := transactor.Within(ctx, func(context.Context) error { return want })
		if !errors.Is(nestedErr, want) {
			t.Fatalf("nested error=%v", nestedErr)
		}
		return nil
	})
	if !errors.Is(err, ErrRollbackOnly) || !errors.Is(err, want) {
		t.Fatalf("outer error=%v", err)
	}
	if state.commits != 0 || state.rollbacks != 1 {
		t.Fatalf("commits=%d rollbacks=%d", state.commits, state.rollbacks)
	}
}

func TestWithinRollsBackAndRepanics(t *testing.T) {
	db, state := openScriptedDB(t)
	transactor := New(db)
	want := "panic marker"

	defer func() {
		if recovered := recover(); recovered != want {
			t.Fatalf("recovered=%v", recovered)
		}
		if state.commits != 0 || state.rollbacks != 1 {
			t.Fatalf("commits=%d rollbacks=%d", state.commits, state.rollbacks)
		}
	}()
	_ = transactor.Within(context.Background(), func(context.Context) error { panic(want) })
}

func TestWithinReturnsBeginAndCommitFailures(t *testing.T) {
	t.Run("begin", func(t *testing.T) {
		want := errors.New("begin failure")
		db, state := openScriptedDB(t)
		state.beginErr = want
		err := New(db).Within(context.Background(), func(context.Context) error { return nil })
		if !errors.Is(err, want) || state.commits != 0 || state.rollbacks != 0 {
			t.Fatalf("error=%v commits=%d rollbacks=%d", err, state.commits, state.rollbacks)
		}
	})

	t.Run("commit", func(t *testing.T) {
		want := errors.New("commit failure")
		db, state := openScriptedDB(t)
		state.commitErr = want
		called := false
		err := New(db).Within(context.Background(), func(ctx context.Context) error {
			return AfterCommit(ctx, func(context.Context) error { called = true; return nil })
		})
		if !errors.Is(err, want) || state.commits != 1 || called {
			t.Fatalf("error=%v commits=%d afterCommit=%t", err, state.commits, called)
		}
	})
}

func TestAfterCommitFailureReportsCommittedState(t *testing.T) {
	db, state := openScriptedDB(t)
	want := errors.New("cache invalidation failed")
	err := New(db).Within(context.Background(), func(ctx context.Context) error {
		return AfterCommit(ctx, func(context.Context) error { return want })
	})
	if !errors.Is(err, ErrAfterCommit) || !errors.Is(err, want) {
		t.Fatalf("Within error=%v", err)
	}
	var committedErr *AfterCommitError
	if !errors.As(err, &committedErr) {
		t.Fatalf("error type=%T", err)
	}
	if state.commits != 1 || state.rollbacks != 0 {
		t.Fatalf("commits=%d rollbacks=%d", state.commits, state.rollbacks)
	}
}

func TestCompletedContextRejectsNewNestedScope(t *testing.T) {
	db, _ := openScriptedDB(t)
	transactor := New(db)
	var completedCtx context.Context
	if err := transactor.Within(context.Background(), func(ctx context.Context) error {
		completedCtx = ctx
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	called := false
	err := transactor.Within(completedCtx, func(context.Context) error {
		called = true
		return nil
	})
	if !errors.Is(err, ErrTransactionComplete) || called {
		t.Fatalf("error=%v called=%t", err, called)
	}
}

func TestWithExistingJoinsTransactionWithoutOwningIt(t *testing.T) {
	db, state := openScriptedDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	transactor := New(db)
	want := errors.New("nested failure")
	err = WithExisting(context.Background(), tx, func(ctx context.Context) error {
		if Executor(ctx, db) != tx {
			t.Fatal("Executor did not return the existing transaction")
		}
		_ = transactor.Within(ctx, func(context.Context) error { return want })
		return nil
	})
	if !errors.Is(err, ErrRollbackOnly) || !errors.Is(err, want) {
		t.Fatalf("WithExisting error=%v", err)
	}
	if state.commits != 0 || state.rollbacks != 0 {
		t.Fatalf("WithExisting owned transaction: commits=%d rollbacks=%d", state.commits, state.rollbacks)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if state.rollbacks != 1 {
		t.Fatalf("caller rollback count=%d", state.rollbacks)
	}
}

func TestWithExistingRejectsAfterCommitCallbacks(t *testing.T) {
	db, _ := openScriptedDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	err = WithExisting(context.Background(), tx, func(ctx context.Context) error {
		return AfterCommit(ctx, func(context.Context) error { return nil })
	})
	if !errors.Is(err, ErrExternalAfterCommit) {
		t.Fatalf("WithExisting error=%v", err)
	}
}

var scriptedDriverID atomic.Uint64

type scriptedState struct {
	sync.Mutex
	beginErr, commitErr, rollbackErr error
	commits, rollbacks               int
}

func openScriptedDB(t *testing.T) (*sql.DB, *scriptedState) {
	t.Helper()
	state := &scriptedState{}
	name := fmt.Sprintf("transaction-scripted-%d", scriptedDriverID.Add(1))
	sql.Register(name, scriptedDriver{state: state})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, state
}

type scriptedDriver struct{ state *scriptedState }

func (d scriptedDriver) Open(string) (driver.Conn, error) { return &scriptedConn{state: d.state}, nil }

type scriptedConn struct{ state *scriptedState }

func (c *scriptedConn) Prepare(string) (driver.Stmt, error) { return nil, io.EOF }
func (c *scriptedConn) Close() error                        { return nil }
func (c *scriptedConn) Begin() (driver.Tx, error)           { return c.begin() }
func (c *scriptedConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return c.begin()
}
func (c *scriptedConn) begin() (driver.Tx, error) {
	c.state.Lock()
	defer c.state.Unlock()
	if c.state.beginErr != nil {
		return nil, c.state.beginErr
	}
	return &scriptedTx{state: c.state}, nil
}

type scriptedTx struct{ state *scriptedState }

func (tx *scriptedTx) Commit() error {
	tx.state.Lock()
	defer tx.state.Unlock()
	tx.state.commits++
	return tx.state.commitErr
}
func (tx *scriptedTx) Rollback() error {
	tx.state.Lock()
	defer tx.state.Unlock()
	tx.state.rollbacks++
	return tx.state.rollbackErr
}
