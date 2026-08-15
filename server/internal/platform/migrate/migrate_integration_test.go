//go:build integration

package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestCommerceReaderSearchProjectionMigrationScenarios(t *testing.T) {
	adminDSN := strings.TrimSpace(os.Getenv("MOONBOOK_MIGRATION_TEST_ADMIN_DSN"))
	if adminDSN == "" {
		t.Skip("MOONBOOK_MIGRATION_TEST_ADMIN_DSN 未配置")
	}
	adminDB, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer adminDB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	for _, scenario := range []struct {
		name    string
		prepare func(*testing.T, context.Context, *sql.DB)
		verify  func(*testing.T, context.Context, *sql.DB)
	}{
		{name: "empty database", verify: verifyEmptyProjection},
		{name: "existing accounts", prepare: prepareExistingAccounts, verify: verifyExistingAccountProjection},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			databaseName := fmt.Sprintf("moonbook_migration_it_%d", time.Now().UnixNano())
			if _, err := adminDB.ExecContext(ctx, `CREATE DATABASE "`+databaseName+`"`); err != nil {
				t.Fatal(err)
			}
			databaseDSN := dsnWithDatabase(t, adminDSN, databaseName)
			db, err := sql.Open("pgx", databaseDSN)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = db.Close()
				_, _ = adminDB.ExecContext(context.Background(), `DROP DATABASE IF EXISTS "`+databaseName+`" WITH (FORCE)`)
			})

			provider, err := NewProvider(db)
			if err != nil {
				t.Fatal(err)
			}
			if scenario.prepare != nil {
				results, err := provider.UpTo(ctx, 58)
				if err != nil || len(results) != 58 {
					t.Fatalf("migrate to 58: applied=%d err=%v", len(results), err)
				}
				scenario.prepare(t, ctx, db)
			}
			results, err := provider.Up(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if scenario.prepare == nil && len(results) != 59 || scenario.prepare != nil && len(results) != 1 {
				t.Fatalf("unexpected applied migrations: %d", len(results))
			}
			scenario.verify(t, ctx, db)

			replayed, err := provider.Up(ctx)
			if err != nil || len(replayed) != 0 {
				t.Fatalf("repeat migration: applied=%d err=%v", len(replayed), err)
			}
		})
	}
}

func prepareExistingAccounts(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
INSERT INTO reader_accounts (id, username, nickname, password_hash, status, created_at, updated_at)
VALUES
    (8100000000001, 'projection-alpha', 'Alpha', 'fixture', 'enabled', '2026-01-01T00:00:00Z', '2026-01-02T00:00:00Z'),
    (8100000000002, 'projection-beta', '', 'fixture', 'disabled', '2026-02-01T00:00:00Z', '2026-02-02T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
}

func verifyEmptyProjection(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM commerce_reader_search_projection`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("empty projection count=%d err=%v", count, err)
	}
}

func verifyExistingAccountProjection(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	var accountCount, projectionCount, differenceCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_accounts`).Scan(&accountCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM commerce_reader_search_projection`).Scan(&projectionCount); err != nil {
		t.Fatal(err)
	}
	if accountCount != 2 || projectionCount != accountCount {
		t.Fatalf("account count=%d projection count=%d", accountCount, projectionCount)
	}
	if err := db.QueryRowContext(ctx, `
SELECT count(*) FROM (
    (SELECT id, username, nickname, status, created_at, updated_at FROM reader_accounts
     EXCEPT
     SELECT reader_id, username, nickname, status, created_at, updated_at FROM commerce_reader_search_projection)
    UNION ALL
    (SELECT reader_id, username, nickname, status, created_at, updated_at FROM commerce_reader_search_projection
     EXCEPT
     SELECT id, username, nickname, status, created_at, updated_at FROM reader_accounts)
) differences`).Scan(&differenceCount); err != nil || differenceCount != 0 {
		t.Fatalf("projection differences=%d err=%v", differenceCount, err)
	}
}

func dsnWithDatabase(t *testing.T, dsn, databaseName string) string {
	t.Helper()
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/" + databaseName
	return parsed.String()
}
