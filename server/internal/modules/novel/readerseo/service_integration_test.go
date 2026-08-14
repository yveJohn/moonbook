package readerseo

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestConfigLifecycleWithPostgres(t *testing.T) {
	dsn := os.Getenv("MOONBOOK_READER_SEO_TEST_DSN")
	if dsn == "" {
		t.Skip("MOONBOOK_READER_SEO_TEST_DSN is not configured")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	service := NewService(db)
	original, err := service.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, restoreErr := service.Update(context.Background(), Input{
			SEOEnabled: original.SEOEnabled, IndexingEnabled: original.IndexingEnabled,
			SitemapEnabled: original.SitemapEnabled, SiteName: original.SiteName, SiteURL: original.SiteURL,
			DefaultDescription: original.DefaultDescription, HomeTitle: original.HomeTitle,
			HomeDescription: original.HomeDescription, BooksTitleTemplate: original.BooksTitleTemplate,
			BooksDescriptionTemplate: original.BooksDescriptionTemplate, BookTitleTemplate: original.BookTitleTemplate,
			BookDescriptionTemplate: original.BookDescriptionTemplate,
		})
		if restoreErr != nil {
			t.Errorf("restore config: %v", restoreErr)
		}
	}()

	input := validInput()
	input.SEOEnabled = false
	input.SiteName = "集成测试书城"
	updated, err := service.Update(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != 1 || updated.SEOEnabled || updated.SiteName != "集成测试书城" || updated.SiteURL != "https://ybsc.me" {
		t.Fatalf("updated config=%+v", updated)
	}
	read, err := service.Get(ctx)
	if err != nil || read.ID != updated.ID || read.UpdatedAt.Before(read.CreatedAt) {
		t.Fatalf("read config=%+v err=%v", read, err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM novel_reader_seo_config`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("config row count=%d", count)
	}
}
