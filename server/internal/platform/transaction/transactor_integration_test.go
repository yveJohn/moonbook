//go:build integration

package transaction

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestTransactorCommitRollbackAndConcurrencyWithPostgres(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	prefix := fmt.Sprintf("transaction-it-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM migration_errors WHERE migration_name LIKE $1`, prefix+"%")
	})
	transactor := New(db)

	insert := func(ctx context.Context, name string) error {
		_, err := Executor(ctx, db).ExecContext(ctx, `INSERT INTO migration_errors(migration_name,stage,error_code,error_message,retryable) VALUES($1,'transaction','TEST','fixture',false)`, name)
		return err
	}
	if err := transactor.Within(ctx, func(txCtx context.Context) error {
		return insert(txCtx, prefix+"-commit")
	}); err != nil {
		t.Fatal(err)
	}
	wantRollback := errors.New("force rollback")
	if err := transactor.Within(ctx, func(txCtx context.Context) error {
		if err := insert(txCtx, prefix+"-rollback"); err != nil {
			return err
		}
		return wantRollback
	}); !errors.Is(err, wantRollback) {
		t.Fatalf("rollback error=%v", err)
	}
	if err := transactor.Within(ctx, func(txCtx context.Context) error {
		_ = transactor.Within(txCtx, func(nestedCtx context.Context) error {
			if err := insert(nestedCtx, prefix+"-nested"); err != nil {
				return err
			}
			return wantRollback
		})
		return nil
	}); !errors.Is(err, ErrRollbackOnly) {
		t.Fatalf("nested rollback-only error=%v", err)
	}

	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func(i int) {
			errs <- transactor.Within(ctx, func(txCtx context.Context) error {
				return insert(txCtx, fmt.Sprintf("%s-concurrent-%d", prefix, i))
			})
		}(i)
	}
	for i := 0; i < 4; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}

	var committed, rolledBack, nested, concurrent int
	if err := db.QueryRowContext(ctx, `SELECT
		count(*) FILTER (WHERE migration_name=$1),
		count(*) FILTER (WHERE migration_name=$2),
		count(*) FILTER (WHERE migration_name=$3),
		count(*) FILTER (WHERE migration_name LIKE $4)
		FROM migration_errors WHERE migration_name LIKE $5`,
		prefix+"-commit", prefix+"-rollback", prefix+"-nested", prefix+"-concurrent-%", prefix+"%").
		Scan(&committed, &rolledBack, &nested, &concurrent); err != nil {
		t.Fatal(err)
	}
	if committed != 1 || rolledBack != 0 || nested != 0 || concurrent != 4 {
		t.Fatalf("committed=%d rolledBack=%d nested=%d concurrent=%d", committed, rolledBack, nested, concurrent)
	}
}
