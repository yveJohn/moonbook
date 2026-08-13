package legacymigrate

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestNovelBooksMigrationWithMySQLAndPostgres(t *testing.T) {
	mysqlDSN := os.Getenv("MOONBOOK_LEGACY_BOOK_TEST_DSN")
	postgresDSN := os.Getenv("MOONBOOK_MIGRATION_TEST_DSN")
	if mysqlDSN == "" || postgresDSN == "" {
		t.Skip("MOONBOOK_LEGACY_BOOK_TEST_DSN and MOONBOOK_MIGRATION_TEST_DSN are not configured")
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
	defer target.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	migration := "novel-books-integration-" + strings.ReplaceAll(time.Now().UTC().Format("150405.000000000"), ".", "")
	if _, err := target.ExecContext(ctx, `DELETE FROM novel_book_tags WHERE book_id BETWEEN 9007199254740996 AND 9007199254740999 OR book_id=9223372036854775807`); err != nil {
		t.Fatal(err)
	}
	if _, err := target.ExecContext(ctx, `DELETE FROM novel_book_sub_categories WHERE book_id BETWEEN 9007199254740996 AND 9007199254740999 OR book_id=9223372036854775807`); err != nil {
		t.Fatal(err)
	}
	if _, err := target.ExecContext(ctx, `DELETE FROM novel_books WHERE id BETWEEN 9007199254740996 AND 9007199254740999`); err != nil {
		t.Fatal(err)
	}
	if _, err := target.ExecContext(ctx, `INSERT INTO novel_categories(id,code,name,kind,source) VALUES
		(9007199254740993,'fantasy','玄幻','primary','legacy_dict'),
		(9007199254740994,'system','系统','sub','legacy_dict')
		ON CONFLICT (id) DO UPDATE SET code=EXCLUDED.code,name=EXCLUDED.name,kind=EXCLUDED.kind,deleted_at=NULL`); err != nil {
		t.Fatal(err)
	}
	if _, err := target.ExecContext(ctx, `INSERT INTO novel_authors(id,pen_name,normalized_name,status,source,legacy_book_author_id) VALUES
		(9007199254740993,'现行作者','现行作者','active','legacy_book',9007199254740993)
		ON CONFLICT (id) DO UPDATE SET pen_name=EXCLUDED.pen_name,normalized_name=EXCLUDED.normalized_name,status='active',deleted_at=NULL`); err != nil {
		t.Fatal(err)
	}
	var noIDAuthor int64
	if err := target.QueryRowContext(ctx, `INSERT INTO novel_authors(pen_name,normalized_name,status,source) VALUES ('无ID作者','无id作者','active','legacy_book') RETURNING id`).Scan(&noIDAuthor); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = target.ExecContext(context.Background(), `DELETE FROM novel_book_tags WHERE book_id BETWEEN 9007199254740996 AND 9007199254740999 OR book_id IN (SELECT id FROM novel_books WHERE book_name=$1)`, migration)
		_, _ = target.ExecContext(context.Background(), `DELETE FROM novel_book_sub_categories WHERE book_id BETWEEN 9007199254740996 AND 9007199254740999`)
		_, _ = target.ExecContext(context.Background(), `DELETE FROM novel_books WHERE id BETWEEN 9007199254740996 AND 9007199254740999 OR book_name=$1`, migration)
		_, _ = target.ExecContext(context.Background(), `DELETE FROM migration_errors WHERE migration_name=$1`, migration)
		_, _ = target.ExecContext(context.Background(), `DELETE FROM migration_checkpoints WHERE migration_name=$1`, migration)
		_, _ = target.ExecContext(context.Background(), `DELETE FROM novel_authors WHERE id=$1`, noIDAuthor)
	}()
	runner, err := NewRunner(source, target, migration, 2)
	if err != nil {
		t.Fatal(err)
	}
	runner.verifySource = func(context.Context, *sql.DB) error { return nil }
	stages := []Stage{NovelBooksStage{}, NovelBookSubCategoriesStage{}}
	if err := runner.Run(ctx, stages...); err != nil {
		t.Fatal(err)
	}
	if err := runner.Run(ctx, stages...); err != nil {
		t.Fatalf("idempotent rerun: %v", err)
	}
	assertScalar := func(query string, want int) {
		t.Helper()
		var got int
		if err := target.QueryRowContext(ctx, query).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("query %q = %d, want %d", query, got, want)
		}
	}
	assertScalar(`SELECT count(*) FROM novel_books WHERE id IN (9007199254740996,9007199254740997)`, 2)
	assertScalar(`SELECT count(*) FROM novel_books WHERE id=9007199254740996 AND book_status='serializing' AND publish_status='published' AND fixed_price_coin=9007199254740993 AND author_id=9007199254740993 AND featured`, 1)
	assertScalar(`SELECT count(*) FROM novel_books WHERE id=9007199254740997 AND book_status='completed' AND publish_status='deprecated' AND charge_mode='login_free' AND fixed_price_coin IS NULL`, 1)
	assertScalar(`SELECT count(*) FROM novel_book_sub_categories WHERE book_id=9007199254740996 AND category_code='system'`, 1)
	assertScalar("SELECT count(*) FROM migration_errors WHERE migration_name='"+migration+"' AND error_code IN ('MISSING_CATEGORY','INVALID_BOOK_STATUS','INVALID_SUB_CATEGORY','MISSING_BOOK')", 4)
	assertScalar("SELECT count(*) FROM migration_checkpoints WHERE migration_name='"+migration+"' AND (metadata->>'done')::boolean", len(stages))
	var generatedID int64
	if err := target.QueryRowContext(ctx, `INSERT INTO novel_books(primary_category_id,category_code,category_name,book_name,author_id,author_name) SELECT primary_category_id,category_code,category_name,$1,author_id,author_name FROM novel_books WHERE id=9007199254740996 RETURNING id`, migration).Scan(&generatedID); err != nil {
		t.Fatal(err)
	}
	if generatedID <= 9007199254740997 {
		t.Fatalf("novel_books identity did not advance: %d", generatedID)
	}
}
