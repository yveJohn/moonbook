package legacymigrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/readerseo"
)

type NovelReaderSEOStage struct{}

func (NovelReaderSEOStage) Name() string { return "novel-reader-seo" }

type legacyReaderSEO struct {
	id                                          int64
	seoEnabled, indexingEnabled, sitemapEnabled int
	siteName, siteURL                           string
	defaultDescription, homeTitle               string
	homeDescription, booksTitleTemplate         string
	booksDescriptionTemplate, bookTitleTemplate string
	bookDescriptionTemplate                     string
	createdAt, updatedAt                        sql.NullTime
}

func (NovelReaderSEOStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	exists, err := sourceTableExists(ctx, source, "novel_reader_seo_config")
	if err != nil {
		return BatchResult{}, err
	}
	if !exists {
		return missingSEOResult("SOURCE_TABLE_NOT_FOUND", "legacy reader SEO configuration table does not exist"), nil
	}
	lastID, err := parseCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	var fixedExists bool
	if err := source.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_reader_seo_config WHERE id=1)`).Scan(&fixedExists); err != nil {
		return BatchResult{}, fmt.Errorf("check legacy reader SEO configuration: %w", err)
	}
	rows, err := source.QueryContext(ctx, `SELECT id,seo_enabled,indexing_enabled,sitemap_enabled,
		COALESCE(site_name,''),COALESCE(site_url,''),COALESCE(default_description,''),
		COALESCE(home_title,''),COALESCE(home_description,''),COALESCE(books_title_template,''),
		COALESCE(books_description_template,''),COALESCE(book_title_template,''),
		COALESCE(book_description_template,''),create_time,update_time
		FROM novel_reader_seo_config WHERE id>? ORDER BY id LIMIT ?`, lastID, limit)
	if err != nil {
		return BatchResult{}, fmt.Errorf("query legacy reader SEO configuration: %w", err)
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": "novel_reader_seo_config", "fallback": !fixedExists}}
	for rows.Next() {
		var item legacyReaderSEO
		if err := rows.Scan(&item.id, &item.seoEnabled, &item.indexingEnabled, &item.sitemapEnabled,
			&item.siteName, &item.siteURL, &item.defaultDescription, &item.homeTitle,
			&item.homeDescription, &item.booksTitleTemplate, &item.booksDescriptionTemplate,
			&item.bookTitleTemplate, &item.bookDescriptionTemplate, &item.createdAt, &item.updatedAt); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(item.id, 10)
		if item.id != 1 {
			result.Errors = append(result.Errors, seoRecordError(result.NextCursor, "UNEXPECTED_CONFIG_ID", "legacy reader SEO configuration ID must be 1"))
			continue
		}
		input, code, message := normalizeLegacyReaderSEO(item)
		if code != "" {
			result.Errors = append(result.Errors, seoRecordError("1", code, message))
			continue
		}
		if err := updateLegacyReaderSEO(ctx, target, input, item.createdAt, item.updatedAt); err != nil {
			return BatchResult{}, err
		}
		result.Metadata["fallback"] = false
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	if result.Done && !fixedExists {
		result.Processed++
		result.Errors = append(result.Errors, seoRecordError("1", "SOURCE_RECORD_NOT_FOUND", "legacy reader SEO configuration ID 1 does not exist"))
	}
	return result, nil
}

func missingSEOResult(code, message string) BatchResult {
	return BatchResult{
		Processed: 1, Done: true,
		Errors:   []RecordError{seoRecordError("1", code, message)},
		Metadata: map[string]any{"source": "novel_reader_seo_config", "fallback": true},
	}
}

func seoRecordError(sourceID, code, message string) RecordError {
	return RecordError{SourceTable: "novel_reader_seo_config", SourceID: sourceID, Code: code, Message: message, Retryable: false}
}

func normalizeLegacyReaderSEO(item legacyReaderSEO) (readerseo.Input, string, string) {
	for _, value := range []int{item.seoEnabled, item.indexingEnabled, item.sitemapEnabled} {
		if value != 0 && value != 1 {
			return readerseo.Input{}, "INVALID_SEO_BOOLEAN", "legacy reader SEO switches must be 0 or 1"
		}
	}
	input, err := readerseo.Normalize(readerseo.Input{
		SEOEnabled: item.seoEnabled == 1, IndexingEnabled: item.indexingEnabled == 1,
		SitemapEnabled: item.sitemapEnabled == 1, SiteName: item.siteName, SiteURL: item.siteURL,
		DefaultDescription: item.defaultDescription, HomeTitle: item.homeTitle,
		HomeDescription: item.homeDescription, BooksTitleTemplate: item.booksTitleTemplate,
		BooksDescriptionTemplate: item.booksDescriptionTemplate, BookTitleTemplate: item.bookTitleTemplate,
		BookDescriptionTemplate: item.bookDescriptionTemplate,
	})
	if err != nil {
		return readerseo.Input{}, "INVALID_SEO_CONFIG", "legacy reader SEO text, URL, or template validation failed"
	}
	return input, "", ""
}

func updateLegacyReaderSEO(ctx context.Context, target *sql.Tx, input readerseo.Input, createdAt, updatedAt sql.NullTime) error {
	result, err := target.ExecContext(ctx, `UPDATE novel_reader_seo_config SET
		seo_enabled=$1,indexing_enabled=$2,sitemap_enabled=$3,site_name=$4,site_url=$5,
		default_description=$6,home_title=$7,home_description=$8,books_title_template=$9,
		books_description_template=$10,book_title_template=$11,book_description_template=$12,
		created_at=COALESCE($13,created_at),updated_at=COALESCE($14,updated_at) WHERE id=1`,
		input.SEOEnabled, input.IndexingEnabled, input.SitemapEnabled, input.SiteName, input.SiteURL,
		input.DefaultDescription, input.HomeTitle, input.HomeDescription, input.BooksTitleTemplate,
		input.BooksDescriptionTemplate, input.BookTitleTemplate, input.BookDescriptionTemplate,
		nullableLegacyTime(createdAt), nullableLegacyTime(updatedAt))
	if err != nil {
		return fmt.Errorf("write target reader SEO configuration: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect target reader SEO configuration update: %w", err)
	}
	if count != 1 {
		return errors.New("target reader SEO default configuration is missing")
	}
	return nil
}

func nullableLegacyTime(value sql.NullTime) any {
	if value.Valid {
		return value.Time.UTC()
	}
	return nil
}
