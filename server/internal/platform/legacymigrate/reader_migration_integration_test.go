package legacymigrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	commerceprovider "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/provider"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/reconcile"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestReaderMigrationWithMySQLAndPostgres(t *testing.T) {
	mysqlDSN := os.Getenv("MOONBOOK_LEGACY_TEST_DSN")
	postgresDSN := os.Getenv("MOONBOOK_MIGRATION_TEST_DSN")
	if mysqlDSN == "" || postgresDSN == "" {
		t.Skip("MOONBOOK_LEGACY_TEST_DSN and MOONBOOK_MIGRATION_TEST_DSN are not configured")
	}
	source, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	target, err := sql.Open("pgx", postgresDSN)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	const categoryID int64 = 9007199254742201
	const authorID int64 = 9007199254742202
	const bookID int64 = 9007199254742001
	const chapterID int64 = 9007199254742101
	readerIDs := []int64{9007199254740993, 9007199254740994, 9007199254740995}
	migration := fmt.Sprintf("reader-migration-integration-%d", time.Now().UnixNano())
	rerunMigration := migration + "-product-rerun"

	cleanupReaderMigrationFixture(t, target, migration, readerIDs, bookID, chapterID, categoryID, authorID)
	cleanupReaderMigrationFixture(t, target, rerunMigration, readerIDs, bookID, chapterID, categoryID, authorID)
	t.Cleanup(func() {
		cleanupReaderMigrationFixture(t, target, migration, readerIDs, bookID, chapterID, categoryID, authorID)
		cleanupReaderMigrationFixture(t, target, rerunMigration, readerIDs, bookID, chapterID, categoryID, authorID)
		target.Close()
	})
	seedReaderMigrationTargets(t, target, categoryID, authorID, bookID, chapterID)

	runner, err := NewRunner(source, target, migration, 2)
	if err != nil {
		t.Fatal(err)
	}
	stages := []Stage{ReaderIdentityStage{ProjectionWriter: commerceprovider.NewReaderSearch(target)}, ReaderCommerceStage{}, ReaderFinanceStage{}, ReaderActivityStage{}}
	if err := runner.Run(ctx, stages...); err != nil {
		t.Fatal(err)
	}
	if err := runner.Run(ctx, stages...); err != nil {
		t.Fatalf("idempotent rerun: %v", err)
	}
	if _, err := target.ExecContext(ctx, `UPDATE commerce_products SET allow_bonus_coin=false,sort_order=99,source_type='manual',source_ref='corrupt' WHERE id=9007199254741201`); err != nil {
		t.Fatal(err)
	}
	if _, err := target.ExecContext(ctx, `UPDATE commerce_products SET allow_bonus_coin=true,duration_days=7,sort_order=99,source_type='manual',source_ref='corrupt' WHERE id=9007199254741202`); err != nil {
		t.Fatal(err)
	}
	rerun, err := NewRunner(source, target, rerunMigration, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := rerun.Run(ctx, ReaderCommerceStage{}); err != nil {
		t.Fatalf("product migration rerun: %v", err)
	}

	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_accounts WHERE id IN ($1,$2)`, []any{readerIDs[0], readerIDs[1]}, 2)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_accounts WHERE id=$1 AND password_algorithm='bcrypt' AND status='enabled'`, []any{readerIDs[0]}, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_accounts WHERE id=$1 AND password_algorithm='md5' AND status='disabled'`, []any{readerIDs[1]}, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_accounts WHERE id=$1`, []any{readerIDs[2]}, 0)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_sessions WHERE reader_id IN ($1,$2)`, []any{readerIDs[0], readerIDs[1]}, 0)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM commerce_reader_search_projection WHERE reader_id IN ($1,$2)`, []any{readerIDs[0], readerIDs[1]}, 2)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM ((SELECT id,username,nickname,status FROM reader_accounts WHERE id IN ($1,$2) EXCEPT SELECT reader_id,username,nickname,status FROM commerce_reader_search_projection WHERE reader_id IN ($1,$2)) UNION ALL (SELECT reader_id,username,nickname,status FROM commerce_reader_search_projection WHERE reader_id IN ($1,$2) EXCEPT SELECT id,username,nickname,status FROM reader_accounts WHERE id IN ($1,$2))) d`, []any{readerIDs[0], readerIDs[1]}, 0)

	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_invite_relations WHERE id=9007199254741102 AND invite_code_id=9007199254741101`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM commerce_products WHERE id=9007199254741201 AND target_id=$1 AND source_type='legacy'`, []any{bookID}, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM commerce_products WHERE id IN (9007199254741202,9007199254741203) AND product_type='membership' AND target_id IS NULL AND source_type='legacy'`, nil, 2)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM commerce_products WHERE id=9007199254741201 AND allow_bonus_coin AND sort_order=3 AND source_ref='9007199254741201'`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM commerce_products WHERE id=9007199254741202 AND NOT allow_bonus_coin AND duration_days=30 AND sort_order=1 AND source_ref='9007199254741202'`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM commerce_membership_grants WHERE id=9007199254741301 AND grant_type='admin' AND status='active' AND source_ref='MG-FIXTURE-1'`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM commerce_entitlements WHERE id=9007199254741401 AND permanent AND source_type='legacy_order' AND source_ref='ORDER-FIXTURE-1'`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM commerce_entitlements WHERE id=9007199254741402 AND NOT permanent AND source_type='legacy' AND status='disabled'`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_book_likes WHERE id=9007199254741501`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_bookshelf_entries WHERE id=9007199254741601 AND last_chapter_id=$1`, []any{chapterID}, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_reading_history WHERE id=9007199254741701 AND chapter_no=12 AND position_type='page' AND position_value=4 AND progress_percent=37.50`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_reading_preferences WHERE id=9007199254741801 AND font_size=24 AND line_height=2.25 AND theme='green' AND reading_mode='scroll'`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_feedback WHERE id=9007199254741901 AND status='replied' AND reply='已处理' AND replied_at IS NOT NULL`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_wallets WHERE reader_id=$1 AND recharge_coin_balance=900 AND bonus_coin_balance=150 AND total_recharge_coin_income=1000 AND total_bonus_coin_income=200 AND total_recharge_coin_expense=100 AND total_bonus_coin_expense=50 AND source_type='legacy'`, []any{readerIDs[0]}, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1 AND source_type='legacy'`, []any{readerIDs[0]}, 4)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_bonus_coin_buckets WHERE id=9007199254743101 AND remaining_amount=150 AND source_kind='invite'`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_purchase_orders WHERE id=9007199254743201 AND order_type='book' AND status='paid' AND source_type='legacy'`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_checkin_reward_rules WHERE id=9007199254743301 AND rule_type='continuous' AND continuous_days=3`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_checkin_records WHERE id=9007199254743401 AND total_reward_coin=30`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_invite_reward_records WHERE id=9007199254743501 AND reward_stage='register' AND reward_coin=200`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_wallet_adjustments WHERE id=9007199254743502 AND ledger_id=9007199254743004`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_recharge_products WHERE id=9007199254743601 AND diamond_amount=1000 AND source_type='legacy'`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_recharge_orders WHERE id=9007199254743701 AND merchant_pid_sha256=$1 AND legacy_payment_credential_id=77 AND legacy_source_ref='9007199254743701'`, []any{sha256Text("legacy-merchant-pid")}, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_recharge_orders WHERE merchant_pid_sha256='legacy-merchant-pid'`, nil, 0)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_payment_callback_logs WHERE id=9007199254743801 AND source_ip_sha256=$1 AND payload_snapshot->>'trade_id'='TRADE-FIXTURE'`, []any{sha256Text("203.0.113.9")}, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_payment_callback_logs WHERE source_ip_sha256 IN ('203.0.113.9','203.0.113.10')`, nil, 0)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_payment_channels WHERE id=1 AND source_type='legacy' AND enabled=false`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_daily_activity WHERE reader_id=$1 AND source_type='legacy'`, []any{readerIDs[0]}, 2)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_daily_activity WHERE reader_id=$1 AND activity_date='2026-08-14' AND first_active_at='2026-08-14 00:01:02+08'`, []any{readerIDs[0]}, 1)
	reconcileReport, err := reconcile.Wallets(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(reconcileReport.Mismatches) != 0 {
		t.Fatalf("wallet reconciliation mismatches: %+v", reconcileReport.Mismatches)
	}

	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM migration_checkpoints WHERE migration_name=$1 AND (metadata->>'done')::boolean`, []any{migration}, 4)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM migration_checkpoints WHERE migration_name=$1 AND stage='reader-identity' AND processed_count=3 AND error_count=1`, []any{migration}, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM migration_checkpoints WHERE migration_name=$1 AND stage='reader-commerce' AND processed_count=15 AND error_count=2`, []any{migration}, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM migration_checkpoints WHERE migration_name=$1 AND stage='reader-finance' AND processed_count=19 AND error_count=2`, []any{migration}, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM migration_checkpoints WHERE migration_name=$1 AND stage='reader-activity' AND processed_count=3 AND error_count=1`, []any{migration}, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM migration_errors WHERE migration_name=$1 AND error_code IN ('INVALID_PASSWORD_HASH','INVALID_MEMBERSHIP_GRANT_TYPE','INVALID_ENTITLEMENT_TYPE','INVALID_PURCHASE_ORDER','INVALID_PAYMENT_CALLBACK')`, []any{migration}, 5)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM migration_errors WHERE migration_name=$1 AND (error_message ILIKE '%not-a-password-hash%' OR error_message ILIKE '%5f4dcc3b%')`, []any{migration}, 0)

	for table, minimum := range map[string]int64{
		"reader_invite_relations":      9007199254741102,
		"reader_book_likes":            9007199254741501,
		"reader_bookshelf_entries":     9007199254741601,
		"reader_reading_history":       9007199254741701,
		"reader_reading_preferences":   9007199254741801,
		"reader_feedback":              9007199254741901,
		"reader_wallet_ledgers":        9007199254743004,
		"reader_bonus_coin_buckets":    9007199254743101,
		"reader_purchase_orders":       9007199254743201,
		"reader_checkin_reward_rules":  9007199254743301,
		"reader_checkin_records":       9007199254743401,
		"reader_invite_reward_records": 9007199254743501,
		"reader_wallet_adjustments":    9007199254743502,
		"reader_recharge_orders":       9007199254743701,
		"reader_payment_callback_logs": 9007199254743801,
	} {
		var lastValue int64
		query := fmt.Sprintf(`SELECT last_value FROM %s_id_seq`, table)
		if err := target.QueryRowContext(ctx, query).Scan(&lastValue); err != nil {
			t.Fatalf("read %s sequence: %v", table, err)
		}
		if lastValue < minimum {
			t.Fatalf("%s sequence = %d, want at least %d", table, lastValue, minimum)
		}
	}

	var sourceCount int
	var tokenVersionSum int64
	if err := source.QueryRowContext(ctx, `SELECT count(*),sum(token_version) FROM reader_user`).Scan(&sourceCount, &tokenVersionSum); err != nil {
		t.Fatal(err)
	}
	if sourceCount != 3 || tokenVersionSum != 27 {
		t.Fatalf("legacy source changed: count=%d tokenVersionSum=%d", sourceCount, tokenVersionSum)
	}
	var sourceLedgerCount int
	var sourceAmountSum int64
	if err := source.QueryRowContext(ctx, `SELECT count(*),sum(amount) FROM reader_wallet_ledger`).Scan(&sourceLedgerCount, &sourceAmountSum); err != nil {
		t.Fatal(err)
	}
	if sourceLedgerCount != 4 || sourceAmountSum != 1350 {
		t.Fatalf("legacy finance source changed: ledgerCount=%d amountSum=%d", sourceLedgerCount, sourceAmountSum)
	}
}

type failingMigrationProjection struct{ err error }

func (writer failingMigrationProjection) UpsertReaderSearchProjection(context.Context, commercecontract.ReaderSearchProjection) error {
	return writer.err
}
func (writer failingMigrationProjection) DeleteReaderSearchProjection(context.Context, int64) error {
	return writer.err
}

func TestReaderIdentityMigrationRollsBackAccountAndCheckpointWhenProjectionFails(t *testing.T) {
	mysqlDSN := os.Getenv("MOONBOOK_LEGACY_TEST_DSN")
	postgresDSN := os.Getenv("MOONBOOK_MIGRATION_TEST_DSN")
	if mysqlDSN == "" || postgresDSN == "" {
		t.Skip("MOONBOOK_LEGACY_TEST_DSN and MOONBOOK_MIGRATION_TEST_DSN are not configured")
	}
	source, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	target, err := sql.Open("pgx", postgresDSN)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	readerIDs := []int64{9007199254740993, 9007199254740994, 9007199254740995}
	migration := fmt.Sprintf("reader-projection-rollback-%d", time.Now().UnixNano())
	cleanupReaderMigrationFixture(t, target, migration, readerIDs, 9007199254742001, 9007199254742101, 9007199254742201, 9007199254742202)
	t.Cleanup(func() {
		cleanupReaderMigrationFixture(t, target, migration, readerIDs, 9007199254742001, 9007199254742101, 9007199254742201, 9007199254742202)
		_ = target.Close()
	})
	runner, err := NewRunner(source, target, migration, 2)
	if err != nil {
		t.Fatal(err)
	}
	stage := ReaderIdentityStage{ProjectionWriter: failingMigrationProjection{err: fmt.Errorf("forced projection failure")}}
	if err := runner.Run(ctx, stage); err == nil {
		t.Fatal("expected projection failure")
	}
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_accounts WHERE id = ANY($1)`, []any{readerIDs}, 0)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM commerce_reader_search_projection WHERE reader_id = ANY($1)`, []any{readerIDs}, 0)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM migration_checkpoints WHERE migration_name=$1`, []any{migration}, 0)
}

func seedReaderMigrationTargets(t *testing.T, db *sql.DB, categoryID, authorID, bookID, chapterID int64) {
	t.Helper()
	ctx := context.Background()
	queries := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO novel_categories(id,code,name,kind) VALUES($1,$2,$3,'primary')`, []any{categoryID, fmt.Sprintf("reader-migration-%d", categoryID), "读者迁移分类"}},
		{`INSERT INTO novel_authors(id,pen_name,normalized_name) VALUES($1,$2,$2)`, []any{authorID, fmt.Sprintf("读者迁移作者%d", authorID)}},
		{`INSERT INTO novel_books(id,primary_category_id,category_code,category_name,book_name,author_id,author_name) VALUES($1,$2,$3,$4,$5,$6,$7)`, []any{bookID, categoryID, fmt.Sprintf("reader-migration-%d", categoryID), "读者迁移分类", fmt.Sprintf("读者迁移书籍%d", bookID), authorID, fmt.Sprintf("读者迁移作者%d", authorID)}},
		{`INSERT INTO novel_chapters(id,book_id,chapter_no,chapter_name) VALUES($1,$2,12,'读者迁移章节')`, []any{chapterID, bookID}},
	}
	for _, item := range queries {
		if _, err := db.ExecContext(ctx, item.query, item.args...); err != nil {
			t.Fatal(err)
		}
	}
}

func cleanupReaderMigrationFixture(t *testing.T, db *sql.DB, migration string, readerIDs []int64, bookID, chapterID, categoryID, authorID int64) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	queries := []struct {
		query string
		args  []any
	}{
		{`DELETE FROM migration_errors WHERE migration_name=$1`, []any{migration}},
		{`DELETE FROM migration_checkpoints WHERE migration_name=$1`, []any{migration}},
		{`DELETE FROM reader_daily_activity WHERE reader_id = ANY($1)`, []any{readerIDs}},
		{`DELETE FROM reader_payment_callback_logs WHERE source_type='legacy' AND source_ref BETWEEN '9007199254743801' AND '9007199254743809'`, nil},
		{`DELETE FROM reader_recharge_orders WHERE legacy_source_ref BETWEEN '9007199254743701' AND '9007199254743709'`, nil},
		{`DELETE FROM reader_wallet_adjustments WHERE source_type='legacy' AND source_ref BETWEEN '9007199254743502' AND '9007199254743509'`, nil},
		{`DELETE FROM reader_invite_reward_records WHERE source_type='legacy' AND source_ref BETWEEN '9007199254743501' AND '9007199254743509'`, nil},
		{`DELETE FROM reader_checkin_records WHERE source_type='legacy' AND source_ref BETWEEN '9007199254743401' AND '9007199254743409'`, nil},
		{`DELETE FROM reader_checkin_reward_rules WHERE source_type='legacy' AND source_ref BETWEEN '9007199254743301' AND '9007199254743309'`, nil},
		{`DELETE FROM reader_purchase_orders WHERE source_type='legacy' AND source_ref BETWEEN '9007199254743201' AND '9007199254743209'`, nil},
		{`DELETE FROM reader_bonus_coin_buckets WHERE source_type='legacy' AND source_ref BETWEEN '9007199254743101' AND '9007199254743109'`, nil},
		{`ALTER TABLE reader_wallet_ledgers DISABLE TRIGGER reader_wallet_ledgers_immutable_update`, nil},
		{`DELETE FROM reader_wallet_ledgers WHERE source_type='legacy' AND source_ref BETWEEN '9007199254743001' AND '9007199254743009'`, nil},
		{`ALTER TABLE reader_wallet_ledgers ENABLE TRIGGER reader_wallet_ledgers_immutable_update`, nil},
		{`DELETE FROM reader_wallets WHERE source_type='legacy' AND reader_id = ANY($1)`, []any{readerIDs}},
		{`DELETE FROM reader_recharge_products WHERE source_type='legacy' AND source_ref BETWEEN '9007199254743601' AND '9007199254743609'`, nil},
		{`UPDATE reader_recharge_settings SET custom_enabled=true,diamonds_per_usdt=7,min_diamond_amount=7,max_diamond_amount=70000,amount_scale=2,rounding_mode='CEILING',source_type='runtime',source_ref=NULL WHERE id=1 AND source_type='legacy'`, nil},
		{`UPDATE reader_payment_channels SET enabled=false,currency='usd',token='usdt',network='tron',source_type='runtime',source_ref=NULL WHERE id=1 AND source_type='legacy'`, nil},
		{`DELETE FROM reader_feedback WHERE reader_id = ANY($1)`, []any{readerIDs}},
		{`DELETE FROM reader_reading_preferences WHERE reader_id = ANY($1)`, []any{readerIDs}},
		{`DELETE FROM reader_reading_history WHERE reader_id = ANY($1)`, []any{readerIDs}},
		{`DELETE FROM reader_bookshelf_entries WHERE reader_id = ANY($1)`, []any{readerIDs}},
		{`DELETE FROM reader_book_likes WHERE reader_id = ANY($1)`, []any{readerIDs}},
		{`DELETE FROM commerce_entitlements WHERE reader_id = ANY($1)`, []any{readerIDs}},
		{`DELETE FROM commerce_membership_grants WHERE reader_id = ANY($1)`, []any{readerIDs}},
		{`DELETE FROM commerce_products WHERE id BETWEEN 9007199254741201 AND 9007199254741209`, nil},
		{`DELETE FROM reader_invite_relations WHERE inviter_reader_id = ANY($1) OR invitee_reader_id = ANY($1)`, []any{readerIDs}},
		{`DELETE FROM reader_invite_codes WHERE inviter_reader_id = ANY($1)`, []any{readerIDs}},
		{`DELETE FROM reader_sessions WHERE reader_id = ANY($1)`, []any{readerIDs}},
		{`DELETE FROM commerce_reader_search_projection WHERE reader_id = ANY($1)`, []any{readerIDs}},
		{`DELETE FROM reader_accounts WHERE id = ANY($1)`, []any{readerIDs}},
		{`DELETE FROM novel_chapters WHERE id=$1`, []any{chapterID}},
		{`DELETE FROM novel_books WHERE id=$1`, []any{bookID}},
		{`DELETE FROM novel_authors WHERE id=$1`, []any{authorID}},
		{`DELETE FROM novel_categories WHERE id=$1`, []any{categoryID}},
	}
	for _, item := range queries {
		if _, err := db.ExecContext(ctx, item.query, item.args...); err != nil && !strings.Contains(err.Error(), "does not exist") {
			t.Fatalf("cleanup reader migration fixture: %v", err)
		}
	}
}

func assertReaderScalar(t *testing.T, db *sql.DB, ctx context.Context, query string, args []any, want int) {
	t.Helper()
	var got int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("query %q = %d, want %d", query, got, want)
	}
}
