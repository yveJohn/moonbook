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
			results, err := provider.UpTo(ctx, 61)
			if err != nil {
				t.Fatal(err)
			}
			if scenario.prepare == nil && len(results) != 61 || scenario.prepare != nil && len(results) != 3 {
				t.Fatalf("unexpected applied migrations: %d", len(results))
			}
			scenario.verify(t, ctx, db)

			replayed, err := provider.UpTo(ctx, 61)
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
		results, err := provider.UpTo(ctx, 60)
		if err != nil || len(results) != 60 {
			t.Fatalf("empty migration: applied=%d err=%v", len(results), err)
		}
		verifyEPUSDTSchema(t, ctx, db)
		replayed, err := provider.UpTo(ctx, 60)
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

		results, err = provider.UpTo(ctx, 60)
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
			if results, err := provider.UpTo(ctx, 60); err == nil || len(results) != 0 || !strings.Contains(err.Error(), "duplicate non-empty "+column) {
				t.Fatalf("duplicate %s migration: applied=%d err=%v", column, len(results), err)
			}
			var current int64
			if err := db.QueryRowContext(ctx, `SELECT max(version_id) FROM moonbook_schema_version WHERE is_applied`).Scan(&current); err != nil || current != 59 {
				t.Fatalf("migration version after failure=%d err=%v", current, err)
			}
		})
	}
}

func TestPaymentChannelBaseURLMigrationScenarios(t *testing.T) {
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
		results, err := provider.UpTo(ctx, 75)
		if err != nil || len(results) != 75 {
			t.Fatalf("empty migration: applied=%d err=%v", len(results), err)
		}
		replayed, err := provider.UpTo(ctx, 75)
		if err != nil || len(replayed) != 0 {
			t.Fatalf("repeat migration: applied=%d err=%v", len(replayed), err)
		}
	})

	t.Run("upgrade disables only incomplete channels", func(t *testing.T) {
		db, ctx := createMigrationTestDatabase(t, adminDB, adminDSN)
		provider, err := NewProvider(db)
		if err != nil {
			t.Fatal(err)
		}
		if results, err := provider.UpTo(ctx, 73); err != nil || len(results) != 73 {
			t.Fatalf("migrate to 73: applied=%d err=%v", len(results), err)
		}
		if _, err := db.ExecContext(ctx, `UPDATE reader_payment_channels SET enabled=true,merchant_pid_ciphertext='pid',secret_ciphertext='secret',create_url='https://pay.example/create',notify_url='https://reader.example/notify',redirect_url='https://reader.example/recharge',health_url='https://pay.example/',sync_url='https://pay.example/sync' WHERE id=1`); err != nil {
			t.Fatal(err)
		}
		if results, err := provider.UpTo(ctx, 75); err != nil || len(results) != 2 {
			t.Fatalf("migrate to 75: applied=%d err=%v", len(results), err)
		}
		var enabled, validated bool
		if err := db.QueryRowContext(ctx, `SELECT enabled FROM reader_payment_channels WHERE id=1`).Scan(&enabled); err != nil || enabled {
			t.Fatalf("incomplete channel enabled=%v err=%v", enabled, err)
		}
		if err := db.QueryRowContext(ctx, `SELECT convalidated FROM pg_constraint WHERE conname='reader_payment_channels_enabled_config_check'`).Scan(&validated); err != nil || !validated {
			t.Fatalf("channel constraint validated=%v err=%v", validated, err)
		}
	})
}

func TestReaderInviteRewardDetailRepairMigrationScenarios(t *testing.T) {
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
		results, err := provider.UpTo(ctx, 76)
		if err != nil || len(results) != 76 {
			t.Fatalf("empty migration: applied=%d err=%v", len(results), err)
		}
		verifyInviteRewardStageIndex(t, ctx, db)
		replayed, err := provider.UpTo(ctx, 76)
		if err != nil || len(replayed) != 0 {
			t.Fatalf("repeat migration: applied=%d err=%v", len(replayed), err)
		}
	})

	t.Run("upgrade backfills only deterministic runtime inviter ledger", func(t *testing.T) {
		db, ctx := createMigrationTestDatabase(t, adminDB, adminDSN)
		provider, err := NewProvider(db)
		if err != nil {
			t.Fatal(err)
		}
		if results, err := provider.UpTo(ctx, 75); err != nil || len(results) != 75 {
			t.Fatalf("migrate to 75: applied=%d err=%v", len(results), err)
		}
		prepareInviteRewardRepairFixture(t, ctx, db, false, false)
		before := inviteRewardRepairProtectedFacts(t, ctx, db)

		results, err := provider.UpTo(ctx, 76)
		if err != nil || len(results) != 1 {
			t.Fatalf("migrate to 76: applied=%d err=%v", len(results), err)
		}
		verifyInviteRewardStageIndex(t, ctx, db)
		var relationID, inviterID, inviteeID, amount int64
		var status, key, sourceType, sourceRef, remark string
		if err := db.QueryRowContext(ctx, `SELECT relation_id,inviter_reader_id,invitee_reader_id,reward_coin,status,idempotency_key,source_type,source_ref,remark FROM reader_invite_reward_records WHERE reward_stage='register' AND invitee_reader_id=9100000000002`).Scan(&relationID, &inviterID, &inviteeID, &amount, &status, &key, &sourceType, &sourceRef, &remark); err != nil {
			t.Fatal(err)
		}
		if relationID != 9100000000004 || inviterID != 9100000000001 || inviteeID != 9100000000002 || amount != 30 || status != "granted" || key != "invite_reward:9100000000002:register" || sourceType != "runtime" || sourceRef != "registration-ledger:9100000000005" || remark != "邀请奖励" {
			t.Fatalf("unexpected repaired reward relation=%d inviter=%d invitee=%d amount=%d status=%q key=%q source=%q/%q remark=%q", relationID, inviterID, inviteeID, amount, status, key, sourceType, sourceRef, remark)
		}
		after := inviteRewardRepairProtectedFacts(t, ctx, db)
		if after.accounts != before.accounts || after.relations != before.relations || after.wallets != before.wallets || after.ledgers != before.ledgers || after.rewards != before.rewards+1 {
			t.Fatalf("protected facts changed before=%+v after=%+v", before, after)
		}
		var unrelated int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_invite_reward_records WHERE source_ref IN ('registration-ledger:9100000000006','registration-ledger:9100000000007')`).Scan(&unrelated); err != nil || unrelated != 0 {
			t.Fatalf("unrelated ledgers repaired=%d err=%v", unrelated, err)
		}
	})

	for _, scenario := range []struct {
		name         string
		duplicate    bool
		inconsistent bool
		wantError    string
	}{
		{name: "duplicate stage facts abort", duplicate: true, wantError: "duplicate invite reward stage facts"},
		{name: "inconsistent runtime ledger aborts", inconsistent: true, wantError: "unreconciled runtime invite registration reward ledger"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db, ctx := createMigrationTestDatabase(t, adminDB, adminDSN)
			provider, err := NewProvider(db)
			if err != nil {
				t.Fatal(err)
			}
			if results, err := provider.UpTo(ctx, 75); err != nil || len(results) != 75 {
				t.Fatalf("migrate to 75: applied=%d err=%v", len(results), err)
			}
			prepareInviteRewardRepairFixture(t, ctx, db, scenario.duplicate, scenario.inconsistent)
			before := inviteRewardRepairProtectedFacts(t, ctx, db)
			if results, err := provider.UpTo(ctx, 76); err == nil || len(results) != 0 || !strings.Contains(err.Error(), scenario.wantError) {
				t.Fatalf("failed migration: applied=%d err=%v", len(results), err)
			}
			after := inviteRewardRepairProtectedFacts(t, ctx, db)
			if after != before {
				t.Fatalf("failed migration changed facts before=%+v after=%+v", before, after)
			}
			var current int64
			if err := db.QueryRowContext(ctx, `SELECT max(version_id) FROM moonbook_schema_version WHERE is_applied`).Scan(&current); err != nil || current != 75 {
				t.Fatalf("migration version after failure=%d err=%v", current, err)
			}
			var indexes int
			if err := db.QueryRowContext(ctx, `SELECT count(*) FROM pg_indexes WHERE schemaname='public' AND indexname='reader_invite_reward_records_invitee_stage_uidx'`).Scan(&indexes); err != nil || indexes != 0 {
				t.Fatalf("failed migration index count=%d err=%v", indexes, err)
			}
		})
	}
}

type inviteRewardRepairCounts struct {
	accounts  int
	relations int
	wallets   int
	ledgers   int
	rewards   int
}

func inviteRewardRepairProtectedFacts(t *testing.T, ctx context.Context, db *sql.DB) inviteRewardRepairCounts {
	t.Helper()
	var counts inviteRewardRepairCounts
	queries := []struct {
		query string
		out   *int
	}{
		{`SELECT count(*) FROM reader_accounts WHERE id BETWEEN 9100000000001 AND 9100000000010`, &counts.accounts},
		{`SELECT count(*) FROM reader_invite_relations WHERE id=9100000000004`, &counts.relations},
		{`SELECT count(*) FROM reader_wallets WHERE reader_id BETWEEN 9100000000001 AND 9100000000010`, &counts.wallets},
		{`SELECT count(*) FROM reader_wallet_ledgers WHERE id BETWEEN 9100000000005 AND 9100000000010`, &counts.ledgers},
		{`SELECT count(*) FROM reader_invite_reward_records WHERE invitee_reader_id=9100000000002`, &counts.rewards},
	}
	for _, item := range queries {
		if err := db.QueryRowContext(ctx, item.query).Scan(item.out); err != nil {
			t.Fatal(err)
		}
	}
	return counts
}

func prepareInviteRewardRepairFixture(t *testing.T, ctx context.Context, db *sql.DB, duplicate, inconsistent bool) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
INSERT INTO reader_accounts(id,username,nickname,password_hash,status) VALUES
    (9100000000001,'invite-repair-inviter','邀请人','fixture','enabled'),
    (9100000000002,'invite-repair-invitee','被邀请人','fixture','enabled');
INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status) VALUES
    (9100000000003,'INVITE-REPAIR',9100000000001,'enabled');
INSERT INTO reader_invite_relations(id,inviter_reader_id,invitee_reader_id,invite_code_id,status) VALUES
    (9100000000004,9100000000001,9100000000002,9100000000003,'active');
INSERT INTO reader_wallets(reader_id,bonus_coin_balance,total_bonus_coin_income) VALUES
    (9100000000001,30,30),(9100000000002,5,5);
INSERT INTO reader_wallet_ledgers(id,reader_id,ledger_no,biz_type,biz_id,direction,coin_type,amount,balance_before,balance_after,remark,idempotency_key,source_type,created_at) VALUES
    (9100000000005,9100000000001,'INVITE-REPAIR-INVITER','invite_reward','9100000000004:9100000000002','income','bonus',30,0,30,'邀请奖励','9100000000004:9100000000002:inviter','runtime','2026-08-31T10:00:00Z'),
    (9100000000006,9100000000002,'INVITE-REPAIR-INVITEE','invite_reward','9100000000004:9100000000002','income','bonus',5,0,5,'注册奖励','9100000000004:9100000000002:invitee','runtime','2026-08-31T10:00:00Z'),
    (9100000000007,9100000000001,'INVITE-REPAIR-LEGACY','invite_reward','9100000000004:9100000000002','income','bonus',30,0,30,'旧奖励','legacy-invite-repair','legacy','2026-08-30T10:00:00Z');`)
	if err != nil {
		t.Fatal(err)
	}
	if duplicate {
		_, err = db.ExecContext(ctx, `INSERT INTO reader_invite_reward_records(id,relation_id,inviter_reader_id,invitee_reader_id,reward_stage,reward_coin,status,idempotency_key,granted_at) VALUES
            (9100000000008,9100000000004,9100000000001,9100000000002,'register',30,'granted','duplicate-register-a',now()),
            (9100000000009,9100000000004,9100000000001,9100000000002,'register',30,'granted','duplicate-register-b',now())`)
	} else if inconsistent {
		_, err = db.ExecContext(ctx, `INSERT INTO reader_invite_reward_records(id,relation_id,inviter_reader_id,invitee_reader_id,reward_stage,reward_coin,status,idempotency_key,granted_at) VALUES
            (9100000000008,9100000000004,9100000000001,9100000000002,'register',31,'granted','invite_reward:9100000000002:register',now())`)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func verifyInviteRewardStageIndex(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	var unique bool
	var definition string
	if err := db.QueryRowContext(ctx, `SELECT i.indisunique,pg_get_indexdef(i.indexrelid) FROM pg_class c JOIN pg_index i ON i.indexrelid=c.oid WHERE c.relname='reader_invite_reward_records_invitee_stage_uidx'`).Scan(&unique, &definition); err != nil {
		t.Fatal(err)
	}
	if !unique || !strings.Contains(definition, "invitee_reader_id, reward_stage") {
		t.Fatalf("unexpected invite reward stage index unique=%v definition=%q", unique, definition)
	}
}

func TestEPUSDTCallbackAttemptAuditMigrationScenarios(t *testing.T) {
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
		results, err := provider.UpTo(ctx, 61)
		if err != nil || len(results) != 61 {
			t.Fatalf("empty migration: applied=%d err=%v", len(results), err)
		}
		verifyCallbackAuditSchema(t, ctx, db)
		replayed, err := provider.UpTo(ctx, 61)
		if err != nil || len(replayed) != 0 {
			t.Fatalf("repeat migration: applied=%d err=%v", len(replayed), err)
		}
	})

	t.Run("upgrade preserves historical callback logs and finance facts", func(t *testing.T) {
		db, ctx := createMigrationTestDatabase(t, adminDB, adminDSN)
		provider, err := NewProvider(db)
		if err != nil {
			t.Fatal(err)
		}
		results, err := provider.UpTo(ctx, 60)
		if err != nil || len(results) != 60 {
			t.Fatalf("migrate to 60: applied=%d err=%v", len(results), err)
		}
		prepareCallbackAuditUpgradeFixture(t, ctx, db)
		before := countProtectedFacts(t, ctx, db)

		results, err = provider.UpTo(ctx, 61)
		if err != nil || len(results) != 1 {
			t.Fatalf("migrate to 61: applied=%d err=%v", len(results), err)
		}
		if after := countProtectedFacts(t, ctx, db); fmt.Sprint(after) != fmt.Sprint(before) {
			t.Fatalf("protected fact counts changed: before=%v after=%v", before, after)
		}
		verifyCallbackAuditSchema(t, ctx, db)
		verifyCallbackAuditUpgradeFixture(t, ctx, db)
	})
}

func TestPlatformJobMonitorMigrationScenarios(t *testing.T) {
	adminDSN := strings.TrimSpace(os.Getenv("MOONBOOK_MIGRATION_TEST_ADMIN_DSN"))
	if adminDSN == "" {
		t.Skip("MOONBOOK_MIGRATION_TEST_ADMIN_DSN 未配置")
	}
	adminDB, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer adminDB.Close()

	for _, scenario := range []struct {
		name      string
		from      int64
		wantCount int
	}{
		{name: "empty database", from: 0, wantCount: 68},
		{name: "upgrade from system config hardening", from: 66, wantCount: 2},
		{name: "upgrade from platform job monitor", from: 67, wantCount: 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db, ctx := createMigrationTestDatabase(t, adminDB, adminDSN)
			provider, err := NewProvider(db)
			if err != nil {
				t.Fatal(err)
			}
			if scenario.from > 0 {
				results, err := provider.UpTo(ctx, scenario.from)
				if err != nil || len(results) != int(scenario.from) {
					t.Fatalf("migrate to %d: applied=%d err=%v", scenario.from, len(results), err)
				}
			}
			results, err := provider.UpTo(ctx, 68)
			if err != nil || len(results) != scenario.wantCount {
				t.Fatalf("migrate to 68: applied=%d err=%v", len(results), err)
			}
			verifyPlatformJobMonitorMigration(t, ctx, db)
			verifyUnusedEmailPluginRemoved(t, ctx, db)
			replayed, err := provider.UpTo(ctx, 68)
			if err != nil || len(replayed) != 0 {
				t.Fatalf("repeat migration: applied=%d err=%v", len(replayed), err)
			}
		})
	}
}

func verifyUnusedEmailPluginRemoved(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	for _, query := range []string{
		`SELECT count(*) FROM sys_apis WHERE path IN ('/email/emailTest','/email/sendEmail') AND method='POST'`,
		`SELECT count(*) FROM casbin_rule WHERE ptype='p' AND v1 IN ('/email/emailTest','/email/sendEmail') AND v2='POST'`,
	} {
		var got int
		if err := db.QueryRowContext(ctx, query).Scan(&got); err != nil || got != 0 {
			t.Fatalf("query %q count=%d want=0 err=%v", query, got, err)
		}
	}
}

func verifyPlatformJobMonitorMigration(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	var menuPath, component string
	if err := db.QueryRowContext(ctx, `SELECT path,component FROM sys_base_menus WHERE id=1724`).Scan(&menuPath, &component); err != nil {
		t.Fatal(err)
	}
	if menuPath != "platformJobs" || component != "view/platform/jobs/index.vue" {
		t.Fatalf("unexpected platform menu path=%q component=%q", menuPath, component)
	}
	var chapterCleanPath string
	if err := db.QueryRowContext(ctx, `SELECT path FROM sys_base_menus WHERE id=1722`).Scan(&chapterCleanPath); err != nil {
		t.Fatal(err)
	}
	if chapterCleanPath != "novelChapterClean" {
		t.Fatalf("menu 1722 changed to %q", chapterCleanPath)
	}
	for _, query := range []struct {
		statement string
		want      int
	}{
		{`SELECT count(*) FROM sys_authority_menus WHERE sys_base_menu_id=1724 AND sys_authority_authority_id=888`, 1},
		{`SELECT count(*) FROM sys_apis WHERE id IN (1820,1821) AND path IN ('/platform/jobs','/platform/jobs/:id') AND method='GET'`, 2},
		{`SELECT count(*) FROM casbin_rule WHERE ptype='p' AND v0='888' AND v1 IN ('/platform/jobs','/platform/jobs/:id') AND v2='GET'`, 2},
	} {
		var got int
		if err := db.QueryRowContext(ctx, query.statement).Scan(&got); err != nil || got != query.want {
			t.Fatalf("query %q count=%d want=%d err=%v", query.statement, got, query.want, err)
		}
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
	for _, table := range []string{"reader_recharge_orders", "reader_payment_callback_logs", "reader_wallets", "reader_wallet_ledgers", "commerce_membership_grants", "commerce_entitlements"} {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		counts[table] = count
	}
	return counts
}

func prepareCallbackAuditUpgradeFixture(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
INSERT INTO reader_accounts (id, username, password_hash, status)
VALUES (8400000000001, 'callback-audit-upgrade', 'fixture', 'enabled');
INSERT INTO reader_wallets (reader_id, recharge_coin_balance) VALUES (8400000000001, 10);
INSERT INTO reader_wallet_ledgers
    (id, reader_id, ledger_no, biz_type, direction, coin_type, amount, balance_before, balance_after)
VALUES (8400000000101, 8400000000001, 'CALLBACK-AUDIT-LEDGER', 'recharge', 'income', 'recharge', 10, 0, 10);
INSERT INTO commerce_membership_grants
    (id, reader_id, grant_type, starts_at, permanent, status, source_ref)
VALUES (8400000000201, 8400000000001, 'legacy', '2026-01-01T00:00:00Z', true, 'active', 'callback-audit');
INSERT INTO commerce_entitlements
    (id, reader_id, entitlement_type, target_id, starts_at, permanent, status, source_ref)
VALUES (8400000000301, 8400000000001, 'membership', 0, '2026-01-01T00:00:00Z', true, 'active', 'callback-audit');
INSERT INTO reader_recharge_orders
    (id, order_no, reader_id, request_id, source_type, diamond_amount, price_usdt, provider, currency, token, network, status)
VALUES (8400000000401, 'CALLBACK-AUDIT-ORDER', 8400000000001, 'callback-audit-request', 'custom', 10, 1.00, 'epusdt', 'usd', 'usdt', 'tron', 'paid');
INSERT INTO reader_payment_callback_logs
    (id, provider, recharge_order_id, merchant_order_no, gateway_trade_id, payload_hash, payload_snapshot, signature_valid, processing_result, failure_reason, response_status, response_body, request_time, source_type, source_ref)
VALUES
    (8400000000501, 'manual', 8400000000401, 'CALLBACK-AUDIT-ORDER', 'manual-trade', repeat('a', 64), NULL, false, 'manual_success', '', 200, 'success', '2026-01-01T00:00:00Z', 'runtime', NULL),
    (8400000000502, 'epusdt', 8400000000401, 'CALLBACK-AUDIT-ORDER', 'sync-trade', repeat('b', 64), '{"trade_id":"sync-trade"}', false, 'sync_pending', 'Payment is pending', 200, 'sync', '2026-01-02T00:00:00Z', 'runtime', NULL),
    (8400000000503, 'epusdt', 8400000000401, 'CALLBACK-AUDIT-ORDER', 'legacy-trade', repeat('c', 64), '{"trade_id":"legacy-trade"}', true, 'success', '', 200, 'success', '2026-01-03T00:00:00Z', 'legacy', '8400000000503');`)
	if err != nil {
		t.Fatal(err)
	}
}

func verifyCallbackAuditSchema(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	for _, column := range []string{"failure_code", "request_id", "trace_id", "payload_bytes", "payload_truncated", "completed_at"} {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='reader_payment_callback_logs' AND column_name=$1)`, column).Scan(&exists); err != nil || !exists {
			t.Fatalf("column %s exists=%t err=%v", column, exists, err)
		}
	}
	for _, index := range []string{
		"reader_payment_callback_logs_result_created_idx",
		"reader_payment_callback_logs_failure_created_idx",
		"reader_payment_callback_logs_response_created_idx",
		"reader_payment_callback_logs_request_time_idx",
		"reader_payment_callback_logs_runtime_received_idx",
	} {
		var exists bool
		if err := db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, "public."+index).Scan(&exists); err != nil || !exists {
			t.Fatalf("index %s exists=%t err=%v", index, exists, err)
		}
	}
}

func verifyCallbackAuditUpgradeFixture(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	rows, err := db.QueryContext(ctx, `
SELECT id, processing_result, response_status, response_body,
       failure_code, request_id, trace_id, payload_bytes, payload_truncated, completed_at
FROM reader_payment_callback_logs
WHERE id BETWEEN 8400000000501 AND 8400000000503
ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	wantResults := []string{"manual_success", "sync_pending", "success"}
	wantBodies := []string{"success", "sync", "success"}
	index := 0
	for rows.Next() {
		var id int64
		var result, responseBody string
		var responseStatus int
		var failureCode, requestID, traceID sql.NullString
		var payloadBytes sql.NullInt64
		var payloadTruncated sql.NullBool
		var completedAt sql.NullTime
		if err := rows.Scan(&id, &result, &responseStatus, &responseBody, &failureCode, &requestID, &traceID, &payloadBytes, &payloadTruncated, &completedAt); err != nil {
			t.Fatal(err)
		}
		if index >= len(wantResults) || result != wantResults[index] || responseStatus != 200 || responseBody != wantBodies[index] ||
			failureCode.Valid || requestID.Valid || traceID.Valid || payloadBytes.Valid || payloadTruncated.Valid || completedAt.Valid {
			t.Fatalf("historical callback changed: id=%d result=%s status=%d body=%s new=%+v/%+v/%+v/%+v/%+v/%+v", id, result, responseStatus, responseBody, failureCode, requestID, traceID, payloadBytes, payloadTruncated, completedAt)
		}
		index++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if index != len(wantResults) {
		t.Fatalf("historical callback count=%d want=%d", index, len(wantResults))
	}
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
