package legacymigrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/readerseo"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestNovelReaderSEOMigrationWithMySQLAndPostgres(t *testing.T) {
	mysqlDSN := os.Getenv("MOONBOOK_LEGACY_SEO_TEST_DSN")
	postgresDSN := os.Getenv("MOONBOOK_MIGRATION_TEST_DSN")
	if mysqlDSN == "" || postgresDSN == "" {
		t.Skip("reader SEO migration integration environment is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
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

	original, err := readerseo.NewService(target).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	migrations := []string{
		fmt.Sprintf("novel-reader-seo-integration-%d-a", time.Now().UnixNano()),
		fmt.Sprintf("novel-reader-seo-integration-%d-b", time.Now().UnixNano()),
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		_, _ = target.ExecContext(cleanupCtx, `UPDATE novel_reader_seo_config SET
			seo_enabled=$1,indexing_enabled=$2,sitemap_enabled=$3,site_name=$4,site_url=$5,
			default_description=$6,home_title=$7,home_description=$8,books_title_template=$9,
			books_description_template=$10,book_title_template=$11,book_description_template=$12,
			created_at=$13,updated_at=$14 WHERE id=1`, original.SEOEnabled, original.IndexingEnabled,
			original.SitemapEnabled, original.SiteName, original.SiteURL, original.DefaultDescription,
			original.HomeTitle, original.HomeDescription, original.BooksTitleTemplate,
			original.BooksDescriptionTemplate, original.BookTitleTemplate, original.BookDescriptionTemplate,
			original.CreatedAt, original.UpdatedAt)
		for _, migration := range migrations {
			_, _ = target.ExecContext(cleanupCtx, `DELETE FROM migration_errors WHERE migration_name=$1`, migration)
			_, _ = target.ExecContext(cleanupCtx, `DELETE FROM migration_checkpoints WHERE migration_name=$1`, migration)
		}
	}()

	run := func(migration string) {
		t.Helper()
		runner, err := NewRunner(source, target, migration, 1)
		if err != nil {
			t.Fatal(err)
		}
		runner.verifySource = func(context.Context, *sql.DB) error { return nil }
		if err := runner.Run(ctx, NovelReaderSEOStage{}); err != nil {
			t.Fatal(err)
		}
	}
	run(migrations[0])
	run(migrations[1])

	var siteName, siteURL string
	var seo, indexing, sitemap bool
	var createdAt, updatedAt time.Time
	if err := target.QueryRowContext(ctx, `SELECT seo_enabled,indexing_enabled,sitemap_enabled,site_name,site_url,created_at,updated_at
		FROM novel_reader_seo_config WHERE id=1`).Scan(&seo, &indexing, &sitemap, &siteName, &siteURL, &createdAt, &updatedAt); err != nil {
		t.Fatal(err)
	}
	if !seo || indexing || !sitemap || siteName != "迁移书城" || siteURL != "https://legacy.example.com" {
		t.Fatalf("migrated config seo=%v indexing=%v sitemap=%v site=%q url=%q", seo, indexing, sitemap, siteName, siteURL)
	}
	if createdAt.UTC().Format("2006-01-02 15:04:05") != "2026-08-01 10:00:00" || updatedAt.UTC().Format("2006-01-02 15:04:05") != "2026-08-02 11:00:00" {
		t.Fatalf("migrated times created=%s updated=%s", createdAt, updatedAt)
	}
	for _, migration := range migrations {
		var errors, processed int
		var done bool
		if err := target.QueryRowContext(ctx, `SELECT error_count,processed_count,(metadata->>'done')::boolean
			FROM migration_checkpoints WHERE migration_name=$1 AND stage='novel-reader-seo'`, migration).Scan(&errors, &processed, &done); err != nil {
			t.Fatal(err)
		}
		if errors != 1 || processed != 2 || !done {
			t.Fatalf("migration=%s errors=%d processed=%d done=%v", migration, errors, processed, done)
		}
		var extraErrors int
		if err := target.QueryRowContext(ctx, `SELECT count(*) FROM migration_errors WHERE migration_name=$1
			AND stage='novel-reader-seo' AND source_id='2' AND error_code='UNEXPECTED_CONFIG_ID'`, migration).Scan(&extraErrors); err != nil {
			t.Fatal(err)
		}
		if extraErrors != 1 {
			t.Fatalf("migration=%s unexpected-ID errors=%d", migration, extraErrors)
		}
	}
	var rows int
	if err := target.QueryRowContext(ctx, `SELECT count(*) FROM novel_reader_seo_config`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("target config rows=%d err=%v", rows, err)
	}
}

func TestNovelReaderSEOFallbackWithInvalidOrMissingSource(t *testing.T) {
	invalidDSN := os.Getenv("MOONBOOK_LEGACY_SEO_INVALID_TEST_DSN")
	missingDSN := os.Getenv("MOONBOOK_LEGACY_SEO_MISSING_TEST_DSN")
	postgresDSN := os.Getenv("MOONBOOK_MIGRATION_TEST_DSN")
	if invalidDSN == "" || missingDSN == "" || postgresDSN == "" {
		t.Skip("reader SEO fallback integration environment is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	target, err := sql.Open("pgx", postgresDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	service := readerseo.NewService(target)
	original, err := service.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	migrations := []string{
		fmt.Sprintf("novel-reader-seo-invalid-%d", time.Now().UnixNano()),
		fmt.Sprintf("novel-reader-seo-missing-%d", time.Now().UnixNano()),
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		_, _ = target.ExecContext(cleanupCtx, `UPDATE novel_reader_seo_config SET
			seo_enabled=$1,indexing_enabled=$2,sitemap_enabled=$3,site_name=$4,site_url=$5,
			default_description=$6,home_title=$7,home_description=$8,books_title_template=$9,
			books_description_template=$10,book_title_template=$11,book_description_template=$12,
			created_at=$13,updated_at=$14 WHERE id=1`, original.SEOEnabled, original.IndexingEnabled,
			original.SitemapEnabled, original.SiteName, original.SiteURL, original.DefaultDescription,
			original.HomeTitle, original.HomeDescription, original.BooksTitleTemplate,
			original.BooksDescriptionTemplate, original.BookTitleTemplate, original.BookDescriptionTemplate,
			original.CreatedAt, original.UpdatedAt)
		for _, migration := range migrations {
			_, _ = target.ExecContext(cleanupCtx, `DELETE FROM migration_errors WHERE migration_name=$1`, migration)
			_, _ = target.ExecContext(cleanupCtx, `DELETE FROM migration_checkpoints WHERE migration_name=$1`, migration)
		}
	}()

	marker := validFallbackInput()
	if _, err := service.Update(ctx, marker); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		dsn, migration, code string
	}{
		{invalidDSN, migrations[0], "INVALID_SEO_CONFIG"},
		{missingDSN, migrations[1], "SOURCE_TABLE_NOT_FOUND"},
	}
	for _, test := range tests {
		source, err := sql.Open("mysql", test.dsn)
		if err != nil {
			t.Fatal(err)
		}
		runner, err := NewRunner(source, target, test.migration, 10)
		if err != nil {
			source.Close()
			t.Fatal(err)
		}
		runner.verifySource = func(context.Context, *sql.DB) error { return nil }
		err = runner.Run(ctx, NovelReaderSEOStage{})
		source.Close()
		if err != nil {
			t.Fatal(err)
		}
		current, err := service.Get(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if current.SiteName != marker.SiteName || current.SiteURL != marker.SiteURL || current.HomeTitle != marker.HomeTitle {
			t.Fatalf("fallback changed target config: %+v", current)
		}
		var errors int
		if err := target.QueryRowContext(ctx, `SELECT count(*) FROM migration_errors WHERE migration_name=$1
			AND stage='novel-reader-seo' AND error_code=$2`, test.migration, test.code).Scan(&errors); err != nil {
			t.Fatal(err)
		}
		if errors != 1 {
			t.Fatalf("migration=%s code=%s errors=%d", test.migration, test.code, errors)
		}
	}
}

func validFallbackInput() readerseo.Input {
	return readerseo.Input{
		SEOEnabled: false, IndexingEnabled: false, SitemapEnabled: false,
		SiteName: "目标保留配置", SiteURL: "https://target-before-fallback.example",
		DefaultDescription: "目标默认描述", HomeTitle: "目标首页标题", HomeDescription: "目标首页描述",
		BooksTitleTemplate: "{siteName}", BooksDescriptionTemplate: "{keyword}{categoryName}",
		BookTitleTemplate: "{bookName}{authorName}", BookDescriptionTemplate: "{bookDesc}{siteName}",
	}
}
