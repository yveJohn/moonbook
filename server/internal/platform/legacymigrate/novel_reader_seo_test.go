package legacymigrate

import (
	"database/sql"
	"testing"
)

func TestNovelReaderSEOStageName(t *testing.T) {
	if got := (NovelReaderSEOStage{}).Name(); got != "novel-reader-seo" {
		t.Fatalf("stage name=%q", got)
	}
}

func TestNormalizeLegacyReaderSEO(t *testing.T) {
	item := legacyReaderSEO{
		id: 1, seoEnabled: 1, indexingEnabled: 1, sitemapEnabled: 0,
		siteName: "月白书城", siteURL: "https://ybsc.me/", defaultDescription: "默认描述",
		homeTitle: "首页", homeDescription: "首页描述", booksTitleTemplate: "{siteName}",
		booksDescriptionTemplate: "{keyword}{categoryName}{subCategoryName}",
		bookTitleTemplate:        "{bookName}{authorName}", bookDescriptionTemplate: "{bookDesc}{categoryName}{siteName}",
		createdAt: sql.NullTime{}, updatedAt: sql.NullTime{},
	}
	input, code, message := normalizeLegacyReaderSEO(item)
	if code != "" || message != "" || input.SiteURL != "https://ybsc.me" || input.SitemapEnabled {
		t.Fatalf("input=%+v code=%q message=%q", input, code, message)
	}
	item.seoEnabled = 2
	if _, code, _ := normalizeLegacyReaderSEO(item); code != "INVALID_SEO_BOOLEAN" {
		t.Fatalf("boolean code=%q", code)
	}
	item.seoEnabled = 1
	item.bookTitleTemplate = "{unknown}"
	if _, code, _ := normalizeLegacyReaderSEO(item); code != "INVALID_SEO_CONFIG" {
		t.Fatalf("template code=%q", code)
	}
}

func TestMissingSEOResultIsValidBatch(t *testing.T) {
	result := missingSEOResult("SOURCE_TABLE_NOT_FOUND", "missing")
	if result.Processed != 1 || len(result.Errors) != 1 || !result.Done || result.Metadata["fallback"] != true {
		t.Fatalf("result=%+v", result)
	}
}
