package legacymigrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

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

	cleanupReaderMigrationFixture(t, target, migration, readerIDs, bookID, chapterID, categoryID, authorID)
	t.Cleanup(func() {
		cleanupReaderMigrationFixture(t, target, migration, readerIDs, bookID, chapterID, categoryID, authorID)
		target.Close()
	})
	seedReaderMigrationTargets(t, target, categoryID, authorID, bookID, chapterID)

	runner, err := NewRunner(source, target, migration, 2)
	if err != nil {
		t.Fatal(err)
	}
	stages := []Stage{ReaderIdentityStage{}, ReaderCommerceStage{}}
	if err := runner.Run(ctx, stages...); err != nil {
		t.Fatal(err)
	}
	if err := runner.Run(ctx, stages...); err != nil {
		t.Fatalf("idempotent rerun: %v", err)
	}

	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_accounts WHERE id IN ($1,$2)`, []any{readerIDs[0], readerIDs[1]}, 2)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_accounts WHERE id=$1 AND password_algorithm='bcrypt' AND status='enabled'`, []any{readerIDs[0]}, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_accounts WHERE id=$1 AND password_algorithm='md5' AND status='disabled'`, []any{readerIDs[1]}, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_accounts WHERE id=$1`, []any{readerIDs[2]}, 0)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_sessions WHERE reader_id IN ($1,$2)`, []any{readerIDs[0], readerIDs[1]}, 0)

	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_invite_relations WHERE id=9007199254741102 AND invite_code_id=9007199254741101`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM commerce_products WHERE id=9007199254741201 AND target_id=$1 AND source_type='legacy'`, []any{bookID}, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM commerce_membership_grants WHERE id=9007199254741301 AND grant_type='admin' AND status='active' AND source_ref='MG-FIXTURE-1'`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM commerce_entitlements WHERE id=9007199254741401 AND permanent AND source_type='legacy_order' AND source_ref='ORDER-FIXTURE-1'`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM commerce_entitlements WHERE id=9007199254741402 AND NOT permanent AND source_type='legacy' AND status='disabled'`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_book_likes WHERE id=9007199254741501`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_bookshelf_entries WHERE id=9007199254741601 AND last_chapter_id=$1`, []any{chapterID}, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_reading_history WHERE id=9007199254741701 AND chapter_no=12 AND position_type='page' AND position_value=4 AND progress_percent=37.50`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_reading_preferences WHERE id=9007199254741801 AND font_size=24 AND line_height=2.25 AND theme='green' AND reading_mode='scroll'`, nil, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM reader_feedback WHERE id=9007199254741901 AND status='replied' AND reply='已处理' AND replied_at IS NOT NULL`, nil, 1)

	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM migration_checkpoints WHERE migration_name=$1 AND (metadata->>'done')::boolean`, []any{migration}, 2)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM migration_checkpoints WHERE migration_name=$1 AND stage='reader-identity' AND processed_count=3 AND error_count=1`, []any{migration}, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM migration_checkpoints WHERE migration_name=$1 AND stage='reader-commerce' AND processed_count=13 AND error_count=2`, []any{migration}, 1)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM migration_errors WHERE migration_name=$1 AND error_code IN ('INVALID_PASSWORD_HASH','INVALID_MEMBERSHIP_GRANT_TYPE','INVALID_ENTITLEMENT_TYPE')`, []any{migration}, 3)
	assertReaderScalar(t, target, ctx, `SELECT count(*) FROM migration_errors WHERE migration_name=$1 AND (error_message ILIKE '%not-a-password-hash%' OR error_message ILIKE '%5f4dcc3b%')`, []any{migration}, 0)

	for table, minimum := range map[string]int64{
		"reader_invite_relations":    9007199254741102,
		"reader_book_likes":          9007199254741501,
		"reader_bookshelf_entries":   9007199254741601,
		"reader_reading_history":     9007199254741701,
		"reader_reading_preferences": 9007199254741801,
		"reader_feedback":            9007199254741901,
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
