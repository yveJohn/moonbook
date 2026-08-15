package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"
	"unicode/utf8"

	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/readerseo"
)

type Public struct {
	db      *sql.DB
	objects *objectstore.Service
	seo     *readerseo.Service
}

var (
	_ novelcontract.PublicBookReader    = (*Public)(nil)
	_ novelcontract.PublicChapterReader = (*Public)(nil)
	_ novelcontract.SEOReader           = (*Public)(nil)
)

func NewPublic(db *sql.DB, objects *objectstore.Service) *Public {
	return &Public{db: db, objects: objects, seo: readerseo.NewService(db)}
}

const publicBookColumns = `b.id,b.book_name,b.author_name,b.description,b.category_code,b.category_name,b.book_status,b.charge_mode,b.word_count,b.like_count,b.visit_count,b.fixed_price_coin,b.last_chapter_id,b.last_chapter_name,b.last_chapter_updated_at,b.featured,b.featured_note`

func (provider *Public) FeaturedBooks(ctx context.Context) ([]novelcontract.Book, error) {
	return provider.listBooks(ctx, "b.featured=true", nil, "b.featured_sort ASC,b.id DESC", 0, 0)
}

func (provider *Public) RandomBooks(ctx context.Context) ([]novelcontract.Book, error) {
	return provider.listBooks(ctx, "", nil, "random()", 20, 0)
}

func (provider *Public) Books(ctx context.Context, keyword, category, subcategory string, page, size int) ([]novelcontract.Book, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}
	args := []any{"%" + strings.TrimSpace(keyword) + "%", strings.TrimSpace(category), strings.TrimSpace(subcategory)}
	where := `b.deleted_at IS NULL AND b.publish_status='published' AND ($1='%%' OR b.book_name ILIKE $1 OR b.author_name ILIKE $1) AND ($2='' OR b.category_code=$2) AND ($3='' OR EXISTS (SELECT 1 FROM novel_book_sub_categories x WHERE x.book_id=b.id AND x.category_code=$3))`
	var total int64
	if err := provider.db.QueryRowContext(ctx, "SELECT count(*) FROM novel_books b WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	items, err := provider.listBooks(ctx, where, args, "b.updated_at DESC,b.id DESC", size, (page-1)*size)
	return items, total, err
}

func (provider *Public) Book(ctx context.Context, id int64) (novelcontract.Book, error) {
	if provider == nil || provider.db == nil || id <= 0 {
		return novelcontract.Book{}, novelcontract.ErrBookNotFound
	}
	book, err := scanPublicBook(provider.db.QueryRowContext(ctx, `SELECT `+publicBookColumns+` FROM novel_books b WHERE b.id=$1 AND b.deleted_at IS NULL AND b.publish_status='published'`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return novelcontract.Book{}, novelcontract.Wrap(novelcontract.ErrBookNotFound, err)
	}
	if err != nil {
		return novelcontract.Book{}, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	books := []novelcontract.Book{book}
	if err := provider.loadSubcategories(ctx, books); err != nil {
		return novelcontract.Book{}, err
	}
	return books[0], nil
}

func (provider *Public) Categories(ctx context.Context, kind string) ([]novelcontract.Category, error) {
	rows, err := provider.db.QueryContext(ctx, `SELECT code,name FROM novel_categories WHERE kind=$1 AND enabled AND deleted_at IS NULL ORDER BY sort,id`, kind)
	if err != nil {
		return nil, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	defer rows.Close()
	items := []novelcontract.Category{}
	for rows.Next() {
		var item novelcontract.Category
		if err := rows.Scan(&item.Code, &item.Name); err != nil {
			return nil, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	return items, nil
}

func (provider *Public) listBooks(ctx context.Context, where string, args []any, order string, limit, offset int) ([]novelcontract.Book, error) {
	if provider == nil || provider.db == nil {
		return nil, novelcontract.ErrUnavailable
	}
	base := `b.deleted_at IS NULL AND b.publish_status='published'`
	if where != "" {
		base += " AND " + where
	}
	query := `SELECT ` + publicBookColumns + ` FROM novel_books b WHERE ` + base + ` ORDER BY ` + order
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	}
	rows, err := provider.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	defer rows.Close()
	items := []novelcontract.Book{}
	for rows.Next() {
		book, err := scanPublicBook(rows)
		if err != nil {
			return nil, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
		}
		items = append(items, book)
	}
	if err := rows.Err(); err != nil {
		return nil, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	if err := provider.loadSubcategories(ctx, items); err != nil {
		return nil, err
	}
	return items, nil
}

type rowScanner interface{ Scan(...any) error }

func scanPublicBook(row rowScanner) (novelcontract.Book, error) {
	var book novelcontract.Book
	err := row.Scan(&book.ID, &book.Name, &book.Author, &book.Description, &book.CategoryCode, &book.CategoryName, &book.BookStatus, &book.ChargeMode, &book.WordCount, &book.LikeCount, &book.VisitCount, &book.FixedPriceCoin, &book.LastChapterID, &book.LastChapterName, &book.LastChapterUpdatedAt, &book.Featured, &book.FeaturedNote)
	return book, err
}

func (provider *Public) loadSubcategories(ctx context.Context, books []novelcontract.Book) error {
	if len(books) == 0 {
		return nil
	}
	placeholders := make([]string, len(books))
	args := make([]any, len(books))
	positions := make(map[int64]int, len(books))
	for index, book := range books {
		placeholders[index] = fmt.Sprintf("$%d", index+1)
		args[index] = book.ID
		positions[book.ID] = index
		books[index].SubCategories = []novelcontract.Category{}
	}
	rows, err := provider.db.QueryContext(ctx, `SELECT book_id,category_code,category_name FROM novel_book_sub_categories WHERE book_id IN (`+strings.Join(placeholders, ",")+") ORDER BY book_id,sort", args...)
	if err != nil {
		return novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	defer rows.Close()
	for rows.Next() {
		var bookID int64
		var category novelcontract.Category
		if err := rows.Scan(&bookID, &category.Code, &category.Name); err != nil {
			return novelcontract.Wrap(novelcontract.ErrUnavailable, err)
		}
		books[positions[bookID]].SubCategories = append(books[positions[bookID]].SubCategories, category)
	}
	if err := rows.Err(); err != nil {
		return novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	return nil
}

func (provider *Public) Chapters(ctx context.Context, bookID int64) ([]novelcontract.Chapter, error) {
	rows, err := provider.db.QueryContext(ctx, `SELECT c.id,c.book_id,c.chapter_no,c.chapter_name,c.word_count,c.updated_at,c.is_vip,c.book_price_coin,b.charge_mode,b.book_name FROM novel_chapters c JOIN novel_books b ON b.id=c.book_id WHERE c.book_id=$1 AND c.deleted_at IS NULL AND c.chapter_status='enabled' AND b.deleted_at IS NULL AND b.publish_status='published' ORDER BY c.chapter_no,c.id`, bookID)
	if err != nil {
		return nil, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	defer rows.Close()
	items := []novelcontract.Chapter{}
	for rows.Next() {
		var chapter novelcontract.Chapter
		if err := rows.Scan(&chapter.ID, &chapter.BookID, &chapter.Number, &chapter.Name, &chapter.WordCount, &chapter.UpdatedAt, &chapter.VIP, &chapter.PriceCoin, &chapter.ChargeMode, &chapter.BookName); err != nil {
			return nil, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
		}
		items = append(items, chapter)
	}
	if err := rows.Err(); err != nil {
		return nil, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	return items, nil
}

func (provider *Public) Chapter(ctx context.Context, id int64) (novelcontract.Chapter, error) {
	if provider == nil || provider.db == nil || id <= 0 {
		return novelcontract.Chapter{}, novelcontract.ErrChapterNotFound
	}
	var chapter novelcontract.Chapter
	err := provider.db.QueryRowContext(ctx, `SELECT c.id,c.book_id,c.chapter_no,c.chapter_name,c.word_count,c.updated_at,c.is_vip,c.book_price_coin,b.charge_mode,b.book_name FROM novel_chapters c JOIN novel_books b ON b.id=c.book_id WHERE c.id=$1 AND c.deleted_at IS NULL AND c.chapter_status='enabled' AND b.deleted_at IS NULL AND b.publish_status='published'`, id).Scan(&chapter.ID, &chapter.BookID, &chapter.Number, &chapter.Name, &chapter.WordCount, &chapter.UpdatedAt, &chapter.VIP, &chapter.PriceCoin, &chapter.ChargeMode, &chapter.BookName)
	if errors.Is(err, sql.ErrNoRows) {
		return novelcontract.Chapter{}, novelcontract.Wrap(novelcontract.ErrChapterNotFound, err)
	}
	if err != nil {
		return novelcontract.Chapter{}, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	var previous, next sql.NullInt64
	err = provider.db.QueryRowContext(ctx, `SELECT
		(SELECT p.id FROM novel_chapters p WHERE p.book_id=c.book_id AND p.deleted_at IS NULL AND p.chapter_status='enabled' AND (p.chapter_no,p.id)<(c.chapter_no,c.id) ORDER BY p.chapter_no DESC,p.id DESC LIMIT 1),
		(SELECT n.id FROM novel_chapters n WHERE n.book_id=c.book_id AND n.deleted_at IS NULL AND n.chapter_status='enabled' AND (n.chapter_no,n.id)>(c.chapter_no,c.id) ORDER BY n.chapter_no,n.id LIMIT 1)
		FROM novel_chapters c WHERE c.id=$1`, chapter.ID).Scan(&previous, &next)
	if err != nil {
		return novelcontract.Chapter{}, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	if previous.Valid {
		chapter.PreviousID = &previous.Int64
	}
	if next.Valid {
		chapter.NextID = &next.Int64
	}
	return chapter, nil
}

func (provider *Public) ChapterContent(ctx context.Context, id int64) (novelcontract.ChapterContent, error) {
	if provider == nil || provider.objects == nil {
		return novelcontract.ChapterContent{}, novelcontract.ErrObjectUnavailable
	}
	chapter, err := provider.Chapter(ctx, id)
	if err != nil {
		return novelcontract.ChapterContent{}, err
	}
	data, object, err := provider.objects.ReadActive(ctx, objectstore.Target{Kind: objectstore.KindChapterContent, BookID: chapter.BookID, OwnerID: chapter.ID})
	if errors.Is(err, objectstore.ErrActiveObjectNotFound) {
		return novelcontract.ChapterContent{}, novelcontract.Wrap(novelcontract.ErrObjectUnavailable, err)
	}
	if errors.Is(err, objectstore.ErrObjectIntegrity) {
		return novelcontract.ChapterContent{}, novelcontract.Wrap(novelcontract.ErrObjectIntegrity, err)
	}
	if err != nil {
		return novelcontract.ChapterContent{}, novelcontract.Wrap(novelcontract.ErrObjectUnavailable, err)
	}
	if !utf8.Valid(data) {
		return novelcontract.ChapterContent{}, novelcontract.ErrObjectEncoding
	}
	return novelcontract.ChapterContent{Chapter: chapter, Text: string(data), Version: object.Version, SHA256: object.SHA256, Bytes: object.ByteSize}, nil
}

func (provider *Public) SEO(ctx context.Context) (novelcontract.SEOConfig, error) {
	if provider == nil || provider.seo == nil {
		return novelcontract.SEOConfig{}, novelcontract.ErrUnavailable
	}
	config, err := provider.seo.Get(ctx)
	if err != nil {
		return novelcontract.SEOConfig{}, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	return novelcontract.SEOConfig{Enabled: config.SEOEnabled, IndexingEnabled: config.IndexingEnabled, SitemapEnabled: config.SitemapEnabled, SiteName: config.SiteName, SiteURL: config.SiteURL, DefaultDescription: config.DefaultDescription, HomeTitle: config.HomeTitle, HomeDescription: config.HomeDescription, BooksTitleTemplate: config.BooksTitleTemplate, BooksDescriptionTemplate: config.BooksDescriptionTemplate, BookTitleTemplate: config.BookTitleTemplate, BookDescriptionTemplate: config.BookDescriptionTemplate}, nil
}

func (provider *Public) Robots(ctx context.Context) (string, error) {
	config, err := provider.SEO(ctx)
	if err != nil {
		return "", err
	}
	if !config.IndexingEnabled || !config.Enabled {
		return "User-agent: *\nDisallow: /\n", nil
	}
	value := "User-agent: *\nAllow: /\nDisallow: /auth/\nDisallow: /me\nDisallow: /shelf\nDisallow: /read/\n"
	if config.SitemapEnabled {
		value += "Sitemap: " + config.SiteURL + "/reader/seo/sitemap.xml\n"
	}
	return value, nil
}

const (
	publicSitemapSize = 49998
	publicXMLHeader   = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"
)

func (provider *Public) Sitemap(ctx context.Context, page int) (string, error) {
	config, err := provider.SEO(ctx)
	if err != nil {
		return "", err
	}
	if !config.Enabled || !config.IndexingEnabled {
		return publicXMLHeader + "<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\"></urlset>\n", nil
	}
	var count int64
	if err := provider.db.QueryRowContext(ctx, `SELECT count(*) FROM novel_books WHERE deleted_at IS NULL AND publish_status='published'`).Scan(&count); err != nil {
		return "", novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	pages := int((count + publicSitemapSize - 1) / publicSitemapSize)
	if pages < 1 {
		pages = 1
	}
	if page < 0 || page > pages || page > 0 && count <= publicSitemapSize {
		return "", novelcontract.ErrBookNotFound
	}
	if page == 0 && count > publicSitemapSize {
		var builder strings.Builder
		builder.WriteString(publicXMLHeader + "<sitemapindex xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
		for current := 1; current <= pages; current++ {
			fmt.Fprintf(&builder, "<sitemap><loc>%s/reader/seo/sitemap-books-%d.xml</loc></sitemap>\n", html.EscapeString(config.SiteURL), current)
		}
		builder.WriteString("</sitemapindex>\n")
		return builder.String(), nil
	}
	offset := page * publicSitemapSize
	if page > 0 {
		offset = (page - 1) * publicSitemapSize
	}
	rows, err := provider.db.QueryContext(ctx, `SELECT id,updated_at FROM novel_books WHERE deleted_at IS NULL AND publish_status='published' ORDER BY updated_at DESC,id DESC LIMIT $1 OFFSET $2`, publicSitemapSize, offset)
	if err != nil {
		return "", novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	defer rows.Close()
	var builder strings.Builder
	builder.WriteString(publicXMLHeader + "<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	if page <= 1 {
		fmt.Fprintf(&builder, "<url><loc>%s/</loc></url>\n<url><loc>%s/books</loc></url>\n", html.EscapeString(config.SiteURL), html.EscapeString(config.SiteURL))
	}
	for rows.Next() {
		var id int64
		var updated time.Time
		if err := rows.Scan(&id, &updated); err != nil {
			return "", novelcontract.Wrap(novelcontract.ErrUnavailable, err)
		}
		fmt.Fprintf(&builder, "<url><loc>%s/books/%d</loc><lastmod>%s</lastmod></url>\n", html.EscapeString(config.SiteURL), id, updated.UTC().Format(time.RFC3339))
	}
	if err := rows.Err(); err != nil {
		return "", novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	builder.WriteString("</urlset>\n")
	return builder.String(), nil
}
