package legacymigrate

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestNovelMetadataMigrationWithMySQLAndPostgres(t *testing.T) {
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
	defer target.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	migration := "novel-metadata-integration-" + time.Now().UTC().Format("150405.000000000")
	runner, err := NewRunner(source, target, migration, 2)
	if err != nil {
		t.Fatal(err)
	}
	stages := []Stage{
		NovelCategoryDictionaryStage{}, LegacyBookCategoryStage{}, NovelBookAuthorStage{},
		LegacyAuthorTableStage{Table: "book_author"}, LegacyAuthorTableStage{Table: "author"},
	}
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
	assertScalar("SELECT count(*) FROM novel_categories WHERE id IN (9007199254740993,9007199254740994,7001)", 3)
	assertScalar("SELECT count(*) FROM novel_authors WHERE id IN (9007199254740993,9007199254740994,9007199254740995)", 3)
	assertScalar("SELECT count(*) FROM novel_authors WHERE id=9007199254740993 AND pen_name='现行作者' AND source='legacy_book' AND legacy_author_id=9007199254740993", 1)
	assertScalar("SELECT count(*) FROM migration_errors WHERE migration_name='"+migration+"' AND error_code IN ('INVALID_AUTHOR','INVALID_CATEGORY')", 2)
	assertScalar("SELECT count(*) FROM migration_checkpoints WHERE migration_name='"+migration+"' AND (metadata->>'done')::boolean", len(stages))
	var generatedCategoryID, generatedAuthorID int64
	if err := target.QueryRowContext(ctx, `INSERT INTO novel_categories (code,name,kind) VALUES ($1,'序列验证','sub') RETURNING id`, migration).Scan(&generatedCategoryID); err != nil {
		t.Fatal(err)
	}
	if err := target.QueryRowContext(ctx, `INSERT INTO novel_authors (pen_name,normalized_name) VALUES ($1,$1) RETURNING id`, migration).Scan(&generatedAuthorID); err != nil {
		t.Fatal(err)
	}
	if generatedCategoryID <= 9007199254740994 || generatedAuthorID <= 9007199254740995 {
		t.Fatalf("identity sequences did not advance: category=%d author=%d", generatedCategoryID, generatedAuthorID)
	}
}
