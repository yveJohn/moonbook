package legacymigrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type NovelBooksStage struct{}

func (NovelBooksStage) Name() string { return "novel-books" }

type legacyBook struct {
	id                                    int64
	workDirection, categoryCode           sql.NullString
	legacyCoverURL, bookName, authorName  string
	authorID                              sql.NullInt64
	description, score                    string
	bookStatus, publishStatus, sourceType string
	featured, featuredSort                int
	featuredNote                          string
	visitCount                            int64
	likeCount, wordCount, commentCount    int
	yesterdayBuy                          int
	lastChapterID                         sql.NullInt64
	lastChapterName                       sql.NullString
	lastChapterUpdatedAt                  sql.NullTime
	chargeMode                            string
	fixedPriceCoin                        sql.NullInt64
	legacyCrawlSourceID                   sql.NullInt64
	legacyCrawlBookID                     sql.NullString
	legacyCrawlLastAt                     sql.NullTime
	legacyCrawlStopped                    int
	createdAt, updatedAt                  sql.NullTime
}

func (NovelBooksStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	for _, table := range []string{"novel_book", "reader_product"} {
		exists, err := sourceTableExists(ctx, source, table)
		if err != nil {
			return BatchResult{}, err
		}
		if !exists {
			return BatchResult{}, fmt.Errorf("required legacy table %s does not exist", table)
		}
	}
	lastID, err := parseCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	rows, err := source.QueryContext(ctx, `
		SELECT b.id,b.work_direction,b.category_code,COALESCE(b.cover_url,''),b.book_name,b.author_id,
			COALESCE(b.author_name,''),COALESCE(b.book_desc,''),CAST(COALESCE(b.score,0) AS CHAR),
			COALESCE(b.book_status,''),COALESCE(b.publish_status,''),COALESCE(b.source_type,''),
			COALESCE(b.featured,0),COALESCE(b.featured_sort,0),COALESCE(b.featured_note,''),
			COALESCE(b.visit_count,0),COALESCE(b.like_count,0),COALESCE(b.word_count,0),
			COALESCE(b.comment_count,0),COALESCE(b.yesterday_buy,0),b.last_chapter_id,b.last_chapter_name,
			b.last_chapter_update_time,COALESCE(b.charge_mode,''),p.price_coin,b.legacy_crawl_source_id,
			b.legacy_crawl_book_id,b.legacy_crawl_last_time,COALESCE(b.legacy_crawl_is_stop,0),
			b.create_time,b.update_time
		FROM novel_book b
		LEFT JOIN reader_product p ON p.product_type='book' AND p.target_id=b.id
		WHERE b.id>? ORDER BY b.id LIMIT ?`, lastID, limit)
	if err != nil {
		return BatchResult{}, err
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": "novel_book"}}
	for rows.Next() {
		var book legacyBook
		if err := rows.Scan(&book.id, &book.workDirection, &book.categoryCode, &book.legacyCoverURL,
			&book.bookName, &book.authorID, &book.authorName, &book.description, &book.score,
			&book.bookStatus, &book.publishStatus, &book.sourceType, &book.featured, &book.featuredSort,
			&book.featuredNote, &book.visitCount, &book.likeCount, &book.wordCount, &book.commentCount,
			&book.yesterdayBuy, &book.lastChapterID, &book.lastChapterName, &book.lastChapterUpdatedAt,
			&book.chargeMode, &book.fixedPriceCoin, &book.legacyCrawlSourceID, &book.legacyCrawlBookID,
			&book.legacyCrawlLastAt, &book.legacyCrawlStopped, &book.createdAt, &book.updatedAt); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(book.id, 10)
		recordError, err := migrateLegacyBook(ctx, target, book)
		if err != nil {
			return BatchResult{}, err
		}
		if recordError != nil {
			result.Errors = append(result.Errors, *recordError)
		}
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	if err := syncIdentitySequence(ctx, target, "novel_books"); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

func migrateLegacyBook(ctx context.Context, target *sql.Tx, book legacyBook) (*RecordError, error) {
	fail := func(code, message string) *RecordError {
		return &RecordError{SourceTable: "novel_book", SourceID: strconv.FormatInt(book.id, 10), Code: code, Message: message}
	}
	book.bookName, book.authorName = strings.TrimSpace(book.bookName), strings.TrimSpace(book.authorName)
	book.description, book.legacyCoverURL = strings.TrimSpace(book.description), strings.TrimSpace(book.legacyCoverURL)
	book.featuredNote, book.sourceType = strings.TrimSpace(book.featuredNote), strings.TrimSpace(book.sourceType)
	if book.id <= 0 || book.bookName == "" || len([]rune(book.bookName)) > 100 || book.authorName == "" || len([]rune(book.authorName)) > 100 {
		return fail("INVALID_BOOK", "book id, name, or author name is invalid"), nil
	}
	if len([]rune(book.description)) > 2000 || len(book.legacyCoverURL) > 500 || len([]rune(book.featuredNote)) > 255 ||
		len([]rune(strings.TrimSpace(book.workDirection.String))) > 32 || len([]rune(strings.TrimSpace(book.categoryCode.String))) > 64 ||
		len([]rune(strings.TrimSpace(book.legacyCrawlBookID.String))) > 128 || len([]rune(strings.TrimSpace(book.lastChapterName.String))) > 255 {
		return fail("INVALID_BOOK", "book text exceeds target field limit"), nil
	}
	score, err := strconv.ParseFloat(strings.TrimSpace(book.score), 64)
	if err != nil || score < 0 || score > 10 {
		return fail("INVALID_SCORE", "score must be between 0 and 10"), nil
	}
	bookStatus, ok := mapLegacyBookStatus(book.bookStatus)
	if !ok {
		return fail("INVALID_BOOK_STATUS", "unsupported book status: "+book.bookStatus), nil
	}
	publishStatus, ok := mapLegacyPublishStatus(book.publishStatus)
	if !ok {
		return fail("INVALID_PUBLISH_STATUS", "unsupported publish status: "+book.publishStatus), nil
	}
	if !validLegacySourceType(book.sourceType) {
		return fail("INVALID_SOURCE_TYPE", "unsupported source type: "+book.sourceType), nil
	}
	if !validLegacyChargeMode(book.chargeMode) || (book.chargeMode == "fixed_price" && (!book.fixedPriceCoin.Valid || book.fixedPriceCoin.Int64 <= 0)) {
		return fail("INVALID_CHARGE_MODE", "charge mode or fixed book price is invalid"), nil
	}
	if book.chargeMode != "fixed_price" {
		book.fixedPriceCoin = sql.NullInt64{}
	}
	if book.featured < 0 || book.featured > 1 || book.featuredSort < 0 || book.visitCount < 0 || book.likeCount < 0 || book.wordCount < 0 || book.commentCount < 0 || book.yesterdayBuy < 0 || book.legacyCrawlStopped < 0 || book.legacyCrawlStopped > 1 {
		return fail("INVALID_BOOK_COUNTER", "book flags, sort, or counters are outside target constraints"), nil
	}
	var categoryID int64
	var categoryName string
	categoryCode := strings.TrimSpace(book.categoryCode.String)
	if categoryCode == "" {
		return fail("MISSING_CATEGORY", "primary category is missing"), nil
	}
	err = target.QueryRowContext(ctx, `SELECT id,name FROM novel_categories WHERE kind='primary' AND code=$1 AND deleted_at IS NULL`, categoryCode).Scan(&categoryID, &categoryName)
	if errors.Is(err, sql.ErrNoRows) {
		return fail("MISSING_CATEGORY", "primary category is missing"), nil
	}
	if err != nil {
		return nil, fmt.Errorf("resolve primary category for book %d: %w", book.id, err)
	}
	authorID, err := resolveLegacyBookAuthor(ctx, target, book.authorID, book.authorName)
	if errors.Is(err, sql.ErrNoRows) {
		return fail("MISSING_AUTHOR", "author is missing"), nil
	}
	if err != nil {
		return nil, fmt.Errorf("resolve author for book %d: %w", book.id, err)
	}
	var duplicateID int64
	err = target.QueryRowContext(ctx, `SELECT id FROM novel_books WHERE book_name=$1 AND author_name=$2 AND deleted_at IS NULL AND id<>$3`, book.bookName, book.authorName, book.id).Scan(&duplicateID)
	if err == nil {
		return fail("DUPLICATE_BOOK", "another legacy id has the same book name and author"), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("check target uniqueness for book %d: %w", book.id, err)
	}
	createdAt, updatedAt := nullableTimeOrNow(book.createdAt), nullableTimeOrNow(book.updatedAt)
	_, err = target.ExecContext(ctx, `INSERT INTO novel_books
		(id,work_direction,primary_category_id,category_code,category_name,legacy_cover_url,book_name,author_id,
		author_name,description,score,book_status,publish_status,source_type,featured,featured_sort,featured_note,
		visit_count,like_count,word_count,comment_count,yesterday_buy,last_chapter_id,last_chapter_name,
		last_chapter_updated_at,charge_mode,fixed_price_coin,legacy_crawl_source_id,legacy_crawl_book_id,
		legacy_crawl_last_at,legacy_crawl_stopped,created_at,updated_at,deleted_at)
		VALUES ($1,NULLIF($2,''),$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,
		$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,NULL)
		ON CONFLICT (id) DO UPDATE SET work_direction=EXCLUDED.work_direction,primary_category_id=EXCLUDED.primary_category_id,
		category_code=EXCLUDED.category_code,category_name=EXCLUDED.category_name,legacy_cover_url=EXCLUDED.legacy_cover_url,
		book_name=EXCLUDED.book_name,author_id=EXCLUDED.author_id,author_name=EXCLUDED.author_name,
		description=EXCLUDED.description,score=EXCLUDED.score,book_status=EXCLUDED.book_status,
		publish_status=EXCLUDED.publish_status,source_type=EXCLUDED.source_type,featured=EXCLUDED.featured,
		featured_sort=EXCLUDED.featured_sort,featured_note=EXCLUDED.featured_note,visit_count=EXCLUDED.visit_count,
		like_count=EXCLUDED.like_count,word_count=EXCLUDED.word_count,comment_count=EXCLUDED.comment_count,
		yesterday_buy=EXCLUDED.yesterday_buy,last_chapter_id=EXCLUDED.last_chapter_id,
		last_chapter_name=EXCLUDED.last_chapter_name,last_chapter_updated_at=EXCLUDED.last_chapter_updated_at,
		charge_mode=EXCLUDED.charge_mode,fixed_price_coin=EXCLUDED.fixed_price_coin,
		legacy_crawl_source_id=EXCLUDED.legacy_crawl_source_id,legacy_crawl_book_id=EXCLUDED.legacy_crawl_book_id,
		legacy_crawl_last_at=EXCLUDED.legacy_crawl_last_at,legacy_crawl_stopped=EXCLUDED.legacy_crawl_stopped,
		created_at=EXCLUDED.created_at,updated_at=EXCLUDED.updated_at,deleted_at=NULL`,
		book.id, strings.TrimSpace(book.workDirection.String), categoryID, categoryCode, categoryName,
		book.legacyCoverURL, book.bookName, authorID, book.authorName, book.description, book.score,
		bookStatus, publishStatus, book.sourceType, book.featured == 1, book.featuredSort, book.featuredNote,
		book.visitCount, book.likeCount, book.wordCount, book.commentCount, book.yesterdayBuy,
		nullableInt64(book.lastChapterID), nullableString(book.lastChapterName), nullableTime(book.lastChapterUpdatedAt),
		book.chargeMode, nullableInt64(book.fixedPriceCoin), nullableInt64(book.legacyCrawlSourceID),
		nullableString(book.legacyCrawlBookID), nullableTime(book.legacyCrawlLastAt), book.legacyCrawlStopped == 1,
		createdAt, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("write target book %d: %w", book.id, err)
	}
	return nil, nil
}

func resolveLegacyBookAuthor(ctx context.Context, target *sql.Tx, legacyID sql.NullInt64, name string) (int64, error) {
	var id int64
	if legacyID.Valid && legacyID.Int64 > 0 {
		err := target.QueryRowContext(ctx, `SELECT id FROM novel_authors WHERE id=$1 AND deleted_at IS NULL`, legacyID.Int64).Scan(&id)
		return id, err
	}
	err := target.QueryRowContext(ctx, `SELECT id FROM novel_authors WHERE normalized_name=$1 AND source='legacy_book' AND legacy_author_id IS NULL AND legacy_book_author_id IS NULL AND deleted_at IS NULL ORDER BY id LIMIT 1`, normalizeLegacyName(name)).Scan(&id)
	return id, err
}

type NovelBookSubCategoriesStage struct{}

func (NovelBookSubCategoriesStage) Name() string { return "novel-book-sub-categories" }

func (NovelBookSubCategoriesStage) RunBatch(ctx context.Context, source *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	exists, err := sourceTableExists(ctx, source, "novel_book_sub_category_rel")
	if err != nil {
		return BatchResult{}, err
	}
	if !exists {
		return BatchResult{Done: true, Metadata: map[string]any{"source": "novel_book_sub_category_rel", "skipped": "table_not_found"}}, nil
	}
	lastID, err := parseCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	rows, err := source.QueryContext(ctx, `SELECT id,book_id,category_code,COALESCE(category_name,''),COALESCE(sort,0) FROM novel_book_sub_category_rel WHERE id>? ORDER BY id LIMIT ?`, lastID, limit)
	if err != nil {
		return BatchResult{}, err
	}
	defer rows.Close()
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": "novel_book_sub_category_rel"}}
	for rows.Next() {
		var id, bookID int64
		var code, sourceName string
		var sort int
		if err := rows.Scan(&id, &bookID, &code, &sourceName, &sort); err != nil {
			return BatchResult{}, err
		}
		result.Processed++
		result.NextCursor = strconv.FormatInt(id, 10)
		var categoryID int64
		var categoryName string
		code, sourceName = strings.TrimSpace(code), strings.TrimSpace(sourceName)
		if bookID <= 0 || sort < 0 || code == "" || len([]rune(code)) > 64 || len([]rune(sourceName)) > 100 {
			result.Errors = append(result.Errors, RecordError{SourceTable: "novel_book_sub_category_rel", SourceID: strconv.FormatInt(id, 10), Code: "INVALID_SUB_CATEGORY", Message: "book id, category, or sort is invalid"})
			continue
		}
		err := target.QueryRowContext(ctx, `SELECT id,name FROM novel_categories WHERE kind='sub' AND code=$1 AND deleted_at IS NULL`, code).Scan(&categoryID, &categoryName)
		if errors.Is(err, sql.ErrNoRows) {
			result.Errors = append(result.Errors, RecordError{SourceTable: "novel_book_sub_category_rel", SourceID: strconv.FormatInt(id, 10), Code: "INVALID_SUB_CATEGORY", Message: "target sub category is missing"})
			continue
		}
		if err != nil {
			return BatchResult{}, err
		}
		var exists bool
		if err := target.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_books WHERE id=$1 AND deleted_at IS NULL)`, bookID).Scan(&exists); err != nil {
			return BatchResult{}, err
		}
		if !exists {
			result.Errors = append(result.Errors, RecordError{SourceTable: "novel_book_sub_category_rel", SourceID: strconv.FormatInt(id, 10), Code: "MISSING_BOOK", Message: "target book is missing"})
			continue
		}
		if _, err := target.ExecContext(ctx, `INSERT INTO novel_book_sub_categories(book_id,category_id,category_code,category_name,sort) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (book_id,category_id) DO UPDATE SET category_code=EXCLUDED.category_code,category_name=EXCLUDED.category_name,sort=EXCLUDED.sort`, bookID, categoryID, code, categoryName, sort); err != nil {
			return BatchResult{}, err
		}
	}
	if err := rows.Err(); err != nil {
		return BatchResult{}, err
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}

func mapLegacyBookStatus(value string) (string, bool) {
	switch strings.TrimSpace(value) {
	case "0":
		return "serializing", true
	case "1":
		return "completed", true
	default:
		return "", false
	}
}

func mapLegacyPublishStatus(value string) (string, bool) {
	switch strings.TrimSpace(value) {
	case "0":
		return "draft", true
	case "1":
		return "published", true
	case "2":
		return "deprecated", true
	default:
		return "", false
	}
}

func validLegacySourceType(value string) bool {
	return value == "manual" || value == "legacy" || value == "txt_import" || value == "forum_crawl"
}

func validLegacyChargeMode(value string) bool {
	return value == "word_charge" || value == "membership_only" || value == "login_free" || value == "fixed_price"
}

func nullableInt64(value sql.NullInt64) any {
	if value.Valid {
		return value.Int64
	}
	return nil
}

func nullableString(value sql.NullString) any {
	if value.Valid && strings.TrimSpace(value.String) != "" {
		return strings.TrimSpace(value.String)
	}
	return nil
}

func nullableTime(value sql.NullTime) any {
	if value.Valid {
		return value.Time
	}
	return nil
}

func nullableTimeOrNow(value sql.NullTime) time.Time {
	if value.Valid {
		return value.Time
	}
	return time.Now().UTC()
}
