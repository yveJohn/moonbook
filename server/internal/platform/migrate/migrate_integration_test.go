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
			if scenario.prepare == nil && len(results) != 60 || scenario.prepare != nil && len(results) != 2 {
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

func TestEPUSDTPaymentCreationMigrationScenarios(t *testing.T) {
	adminDSN := strings.TrimSpace(os.Getenv("MOONBOOK_MIGRATION_TEST_ADMIN_DSN"))
	if adminDSN == "" {
		t.Skip("MOONBOOK_MIGRATION_TEST_ADMIN_DSN 未配置")
	}
	adminDB, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer adminDB.Close()

	t.Run("empty database and replay", func(t *testing.T) {
		db, ctx := createMigrationTestDatabase(t, adminDB, adminDSN)
		provider, err := NewProvider(db)
		if err != nil {
			t.Fatal(err)
		}
		results, err := provider.Up(ctx)
		if err != nil || len(results) != 60 {
			t.Fatalf("empty migration: applied=%d err=%v", len(results), err)
		}
		verifyEPUSDTSchema(t, ctx, db)
		replayed, err := provider.Up(ctx)
		if err != nil || len(replayed) != 0 {
			t.Fatalf("repeat migration: applied=%d err=%v", len(replayed), err)
		}
	})

	t.Run("upgrade preserves facts and converges active orders", func(t *testing.T) {
		db, ctx := createMigrationTestDatabase(t, adminDB, adminDSN)
		provider, err := NewProvider(db)
		if err != nil {
			t.Fatal(err)
		}
		results, err := provider.UpTo(ctx, 59)
		if err != nil || len(results) != 59 {
			t.Fatalf("migrate to 59: applied=%d err=%v", len(results), err)
		}
		prepareEPUSDTUpgradeFixture(t, ctx, db)
		before := countProtectedFacts(t, ctx, db)

		results, err = provider.Up(ctx)
		if err != nil || len(results) != 1 {
			t.Fatalf("migrate to 60: applied=%d err=%v", len(results), err)
		}
		if after := countProtectedFacts(t, ctx, db); fmt.Sprint(after) != fmt.Sprint(before) {
			t.Fatalf("protected fact counts changed: before=%v after=%v", before, after)
		}
		verifyEPUSDTUpgradeFixture(t, ctx, db)
		verifyEPUSDTUniqueIndexes(t, ctx, db)
	})

	for _, column := range []string{"gateway_trade_id", "block_transaction_id"} {
		t.Run("duplicate existing "+column+" aborts", func(t *testing.T) {
			db, ctx := createMigrationTestDatabase(t, adminDB, adminDSN)
			provider, err := NewProvider(db)
			if err != nil {
				t.Fatal(err)
			}
			if results, err := provider.UpTo(ctx, 59); err != nil || len(results) != 59 {
				t.Fatalf("migrate to 59: applied=%d err=%v", len(results), err)
			}
			prepareDuplicateGatewayFixture(t, ctx, db, column)
			if results, err := provider.Up(ctx); err == nil || len(results) != 0 || !strings.Contains(err.Error(), "duplicate non-empty "+column) {
				t.Fatalf("duplicate %s migration: applied=%d err=%v", column, len(results), err)
			}
			var current int64
			if err := db.QueryRowContext(ctx, `SELECT max(version_id) FROM moonbook_schema_version WHERE is_applied`).Scan(&current); err != nil || current != 59 {
				t.Fatalf("migration version after failure=%d err=%v", current, err)
			}
		})
	}
}

func createMigrationTestDatabase(t *testing.T, adminDB *sql.DB, adminDSN string) (*sql.DB, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)
	databaseName := fmt.Sprintf("moonbook_epusdt_migration_it_%d", time.Now().UnixNano())
	if _, err := adminDB.ExecContext(ctx, `CREATE DATABASE "`+databaseName+`"`); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("pgx", dsnWithDatabase(t, adminDSN, databaseName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		_, _ = adminDB.ExecContext(context.Background(), `DROP DATABASE IF EXISTS "`+databaseName+`" WITH (FORCE)`)
	})
	return db, ctx
}

func prepareEPUSDTUpgradeFixture(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
INSERT INTO reader_accounts (id, username, nickname, password_hash, status)
VALUES (8200000000001, 'epusdt-upgrade', 'EPUSDT Upgrade', 'fixture', 'enabled');
INSERT INTO reader_wallets (reader_id, recharge_coin_balance) VALUES (8200000000001, 10);
INSERT INTO reader_wallet_ledgers
    (id, reader_id, ledger_no, biz_type, direction, coin_type, amount, balance_before, balance_after)
VALUES (8200000000101, 8200000000001, 'EPUSDT-MIGRATION-LEDGER', 'recharge', 'income', 'recharge', 10, 0, 10);
INSERT INTO commerce_membership_grants
    (id, reader_id, grant_type, starts_at, permanent, status, source_ref)
VALUES (8200000000201, 8200000000001, 'legacy', '2026-01-01T00:00:00Z', true, 'active', 'epusdt-migration');
INSERT INTO commerce_entitlements
    (id, reader_id, entitlement_type, target_id, starts_at, permanent, status, source_ref)
VALUES (8200000000301, 8200000000001, 'membership', 0, '2026-01-01T00:00:00Z', true, 'active', 'epusdt-migration');
INSERT INTO reader_recharge_orders
    (id, order_no, reader_id, request_id, source_type, diamond_amount, price_usdt, provider, currency, token, network, gateway_trade_id, block_transaction_id, status, created_at, updated_at)
VALUES
    (8200000000401, 'EPUSDT-MIGRATION-OLD', 8200000000001, 'request-old', 'custom', 7, 1.00, 'epusdt', 'usd', 'usdt', 'tron', 'gateway-old', NULL, 'pending', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
    (8200000000402, 'EPUSDT-MIGRATION-TIE-LOW', 8200000000001, 'request-tie-low', 'custom', 14, 2.00, 'epusdt', 'usd', 'usdt', 'tron', NULL, NULL, 'gateway_unknown', '2026-02-01T00:00:00Z', '2026-02-01T00:00:00Z'),
    (8200000000403, 'EPUSDT-MIGRATION-TIE-HIGH', 8200000000001, 'request-tie-high', 'custom', 21, 3.00, 'epusdt', 'usd', 'usdt', 'tron', 'gateway-current', 'block-current', 'creating', '2026-02-01T00:00:00Z', '2026-02-01T00:00:00Z'),
    (8200000000404, 'EPUSDT-MIGRATION-PAID', 8200000000001, 'request-paid', 'custom', 28, 4.00, 'epusdt', 'usd', 'usdt', 'tron', 'gateway-paid', 'block-paid', 'paid', '2026-03-01T00:00:00Z', '2026-03-01T00:00:00Z');`)
	if err != nil {
		t.Fatal(err)
	}
}

func prepareDuplicateGatewayFixture(t *testing.T, ctx context.Context, db *sql.DB, column string) {
	t.Helper()
	if column != "gateway_trade_id" && column != "block_transaction_id" {
		t.Fatalf("unsupported duplicate column %q", column)
	}
	_, err := db.ExecContext(ctx, `
INSERT INTO reader_accounts (id, username, password_hash, status)
VALUES (8300000000001, 'epusdt-duplicate', 'fixture', 'enabled');
INSERT INTO reader_recharge_orders
    (order_no, reader_id, request_id, source_type, diamond_amount, price_usdt, provider, currency, token, network, `+column+`, status)
VALUES
    ('EPUSDT-DUPLICATE-1', 8300000000001, 'duplicate-1', 'custom', 7, 1.00, 'epusdt', 'usd', 'usdt', 'tron', 'duplicate-value', 'paid'),
    ('EPUSDT-DUPLICATE-2', 8300000000001, 'duplicate-2', 'custom', 7, 1.00, 'epusdt', 'usd', 'usdt', 'tron', 'duplicate-value', 'paid');`)
	if err != nil {
		t.Fatal(err)
	}
}

func countProtectedFacts(t *testing.T, ctx context.Context, db *sql.DB) map[string]int {
	t.Helper()
	counts := make(map[string]int)
	for _, table := range []string{"reader_recharge_orders", "reader_wallets", "reader_wallet_ledgers", "commerce_membership_grants", "commerce_entitlements"} {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		counts[table] = count
	}
	return counts
}

func verifyEPUSDTSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	for _, column := range []string{"credential_ref", "merchant_pid_snapshot", "active_reader_id"} {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='reader_recharge_orders' AND column_name=$1)`, column).Scan(&exists); err != nil || !exists {
			t.Fatalf("column %s exists=%t err=%v", column, exists, err)
		}
	}
	for _, index := range []string{"reader_recharge_orders_gateway_trade_id_uidx", "reader_recharge_orders_block_transaction_id_uidx", "reader_recharge_orders_active_reader_id_uidx"} {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, "public."+index).Scan(&exists); err != nil || !exists {
			t.Fatalf("index %s exists=%t err=%v", index, exists, err)
		}
	}
}

func verifyEPUSDTUpgradeFixture(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	rows, err := db.QueryContext(ctx, `SELECT id, status, COALESCE(failure_code, ''), active_reader_id FROM reader_recharge_orders WHERE reader_id=8200000000001 AND status <> 'paid' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type orderState struct {
		id, activeReaderID  int64
		status, failureCode string
	}
	var states []orderState
	for rows.Next() {
		var state orderState
		var activeReaderID sql.NullInt64
		if err := rows.Scan(&state.id, &state.status, &state.failureCode, &activeReaderID); err != nil {
			t.Fatal(err)
		}
		if activeReaderID.Valid {
			state.activeReaderID = activeReaderID.Int64
		}
		states = append(states, state)
	}
	want := []orderState{
		{id: 8200000000401, status: "superseded", failureCode: "ORDER_REPLACED"},
		{id: 8200000000402, status: "superseded", failureCode: "ORDER_REPLACED"},
		{id: 8200000000403, status: "creating", activeReaderID: 8200000000001},
	}
	if fmt.Sprint(states) != fmt.Sprint(want) {
		t.Fatalf("order states=%v want=%v", states, want)
	}
}

func verifyEPUSDTUniqueIndexes(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		`UPDATE reader_recharge_orders SET gateway_trade_id='gateway-current' WHERE id=8200000000401`,
		`UPDATE reader_recharge_orders SET block_transaction_id='block-current' WHERE id=8200000000401`,
		`UPDATE reader_recharge_orders SET active_reader_id=8200000000001 WHERE id=8200000000401`,
	} {
		if _, err := db.ExecContext(ctx, statement); err == nil {
			t.Fatalf("unique index accepted duplicate: %s", statement)
		}
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
