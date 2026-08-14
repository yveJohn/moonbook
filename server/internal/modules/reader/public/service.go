package public

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/catalog"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/readerseo"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

type Service struct {
	db      *sql.DB
	objects *objectstore.Service
	access  *catalog.Service
}

func NewService(db *sql.DB, objects *objectstore.Service, access *catalog.Service) *Service {
	return &Service{db: db, objects: objects, access: access}
}
func notFound(msg string) error { return apperror.New(apperror.CodeNotFound, http.StatusNotFound, msg) }

func (s *Service) scanBook(row *sql.Row) (Book, error) {
	var b Book
	err := row.Scan(&b.ID, &b.Name, &b.Author, &b.Description, &b.CategoryCode, &b.CategoryName, &b.BookStatus, &b.ChargeMode, &b.WordCount, &b.LikeCount, &b.LastChapterID, &b.LastChapterName, &b.LastChapterUpdatedAt, &b.Featured, &b.FeaturedNote)
	return b, err
}

const bookColumns = `b.id,b.book_name,b.author_name,b.description,b.category_code,b.category_name,b.book_status,b.charge_mode,b.word_count,b.like_count,b.last_chapter_id,b.last_chapter_name,b.last_chapter_updated_at,b.featured,b.featured_note`

func (s *Service) relations(ctx context.Context, books []Book) error {
	if len(books) == 0 {
		return nil
	}
	marks := make([]string, len(books))
	args := make([]any, len(books))
	pos := map[int64]int{}
	for i, b := range books {
		marks[i] = fmt.Sprintf("$%d", i+1)
		args[i] = b.ID
		pos[b.ID] = i
		books[i].SubCategories = []Category{}
	}
	rows, e := s.db.QueryContext(ctx, `SELECT book_id,category_code,category_name FROM novel_book_sub_categories WHERE book_id IN (`+strings.Join(marks, ",")+") ORDER BY book_id,sort", args...)
	if e != nil {
		return e
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var c Category
		if e = rows.Scan(&id, &c.Code, &c.Name); e != nil {
			return e
		}
		books[pos[id]].SubCategories = append(books[pos[id]].SubCategories, c)
	}
	return rows.Err()
}
func (s *Service) Featured(ctx context.Context) ([]Book, error) {
	return s.list(ctx, `b.featured=true`, nil, "b.featured_sort ASC,b.id DESC", 0, 0)
}
func (s *Service) Random(ctx context.Context) ([]Book, error) {
	return s.list(ctx, "", nil, "random()", 20, 0)
}
func (s *Service) List(ctx context.Context, keyword, category, sub string, page, size int) ([]Book, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}
	args := []any{"%" + strings.TrimSpace(keyword) + "%", strings.TrimSpace(category), strings.TrimSpace(sub)}
	where := `b.deleted_at IS NULL AND b.publish_status='published' AND ($1='%%' OR b.book_name ILIKE $1 OR b.author_name ILIKE $1) AND ($2='' OR b.category_code=$2) AND ($3='' OR EXISTS (SELECT 1 FROM novel_book_sub_categories x WHERE x.book_id=b.id AND x.category_code=$3))`
	var total int64
	if e := s.db.QueryRowContext(ctx, "SELECT count(*) FROM novel_books b WHERE "+where, args...).Scan(&total); e != nil {
		return nil, 0, e
	}
	items, e := s.list(ctx, where, args, "b.updated_at DESC,b.id DESC", size, (page-1)*size)
	return items, total, e
}
func (s *Service) list(ctx context.Context, where string, args []any, order string, limit, offset int) ([]Book, error) {
	base := `b.deleted_at IS NULL AND b.publish_status='published'`
	if where != "" {
		base += " AND " + where
	}
	q := `SELECT ` + bookColumns + ` FROM novel_books b WHERE ` + base + ` ORDER BY ` + order
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	}
	rows, e := s.db.QueryContext(ctx, q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Book{}
	for rows.Next() {
		var b Book
		if e = rows.Scan(&b.ID, &b.Name, &b.Author, &b.Description, &b.CategoryCode, &b.CategoryName, &b.BookStatus, &b.ChargeMode, &b.WordCount, &b.LikeCount, &b.LastChapterID, &b.LastChapterName, &b.LastChapterUpdatedAt, &b.Featured, &b.FeaturedNote); e != nil {
			return nil, e
		}
		out = append(out, b)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	if e = s.relations(ctx, out); e != nil {
		return nil, e
	}
	return out, nil
}
func (s *Service) Categories(ctx context.Context, kind string) ([]Category, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT code,name FROM novel_categories WHERE kind=$1 AND enabled AND deleted_at IS NULL ORDER BY sort,id`, kind)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if e = rows.Scan(&c.Code, &c.Name); e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Service) Get(ctx context.Context, id int64) (Book, error) {
	b, e := s.scanBook(s.db.QueryRowContext(ctx, `SELECT `+bookColumns+` FROM novel_books b WHERE b.id=$1 AND b.deleted_at IS NULL AND b.publish_status='published'`, id))
	if errors.Is(e, sql.ErrNoRows) {
		return Book{}, notFound("书籍不存在")
	}
	if e != nil {
		return Book{}, e
	}
	books := []Book{b}
	if e = s.relations(ctx, books); e != nil {
		return Book{}, e
	}
	return books[0], nil
}
func (s *Service) Chapters(ctx context.Context, bookID int64, readerID *int64) ([]Chapter, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT c.id,c.book_id,c.chapter_no,c.chapter_name,c.word_count,c.updated_at,c.is_vip,c.book_price_coin,b.charge_mode,b.book_name FROM novel_chapters c JOIN novel_books b ON b.id=c.book_id WHERE c.book_id=$1 AND c.deleted_at IS NULL AND c.chapter_status='enabled' AND b.deleted_at IS NULL AND b.publish_status='published' ORDER BY c.chapter_no,c.id`, bookID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Chapter{}
	for rows.Next() {
		var c Chapter
		if e = rows.Scan(&c.ID, &c.BookID, &c.No, &c.Name, &c.WordCount, &c.UpdatedAt, &c.IsVIP, &c.Price, &c.ChargeMode, &c.BookName); e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	if s.access != nil && len(out) > 0 {
		reqs := make([]catalog.AccessRequest, 0, len(out))
		for _, chapter := range out {
			reqs = append(reqs, catalog.AccessRequest{ReaderID: readerID, BookID: chapter.BookID, ChapterID: chapter.ID, ChargeMode: chapter.ChargeMode, ChapterWordCount: chapter.WordCount})
		}
		results, accessErr := s.access.AccessReaders(ctx, reqs)
		if accessErr != nil {
			return nil, accessErr
		}
		for i := range results {
			out[i].Access = results[i]
		}
	}
	return out, nil
}
func (s *Service) Chapter(ctx context.Context, id int64, readerID *int64) (Chapter, string, objectstore.Object, error) {
	var c Chapter
	e := s.db.QueryRowContext(ctx, `SELECT c.id,c.book_id,c.chapter_no,c.chapter_name,c.word_count,c.updated_at,c.is_vip,c.book_price_coin,b.charge_mode,b.book_name FROM novel_chapters c JOIN novel_books b ON b.id=c.book_id WHERE c.id=$1 AND c.deleted_at IS NULL AND c.chapter_status='enabled' AND b.deleted_at IS NULL AND b.publish_status='published'`, id).Scan(&c.ID, &c.BookID, &c.No, &c.Name, &c.WordCount, &c.UpdatedAt, &c.IsVIP, &c.Price, &c.ChargeMode, &c.BookName)
	if errors.Is(e, sql.ErrNoRows) {
		return c, "", objectstore.Object{}, notFound("章节不存在")
	}
	if e != nil {
		return c, "", objectstore.Object{}, e
	}
	if s.access != nil {
		r, e := s.access.AccessReader(ctx, catalog.AccessRequest{ReaderID: readerID, BookID: c.BookID, ChapterID: c.ID, ChargeMode: c.ChargeMode, ChapterWordCount: c.WordCount})
		if e != nil {
			return c, "", objectstore.Object{}, e
		}
		if !r.Readable {
			return c, "", objectstore.Object{}, accessError(r.AccessReason)
		}
	}
	data, obj, e := s.objects.ReadActive(ctx, objectstore.Target{Kind: objectstore.KindChapterContent, BookID: c.BookID, OwnerID: c.ID})
	if e != nil {
		return c, "", obj, apperror.Wrap(e, apperror.CodeUnavailable, http.StatusServiceUnavailable, "读取章节正文失败")
	}
	if !utf8.Valid(data) {
		return c, "", obj, apperror.New(apperror.CodeInternal, 500, "章节正文编码无效")
	}
	return c, string(data), obj, nil
}
func accessError(reason string) error {
	code := 46102
	msg := "仅限会员阅读"
	switch reason {
	case "login_required":
		code = 46101
		msg = "请先登录后阅读"
	case "book_purchase_required":
		code = 46103
		msg = "请先购买作品"
	case "chapter_purchase_required":
		code = 46104
		msg = "请先购买章节"
	case "unsupported_mode":
		code = 46105
		msg = "作品收费模式不可用"
	}
	return apperror.New(apperror.CodeForbidden, code, msg)
}

func (s *Service) SEO(ctx context.Context) (readerseo.Config, error) {
	return readerseo.NewService(s.db).Get(ctx)
}
func (s *Service) Robots(ctx context.Context) (string, error) {
	c, e := s.SEO(ctx)
	if e != nil {
		return "", e
	}
	if !c.IndexingEnabled || !c.SEOEnabled {
		return "User-agent: *\nDisallow: /\n", nil
	}
	v := "User-agent: *\nAllow: /\nDisallow: /auth/\nDisallow: /me\nDisallow: /shelf\nDisallow: /read/\n"
	if c.SitemapEnabled {
		v += "Sitemap: " + c.SiteURL + "/reader/seo/sitemap.xml\n"
	}
	return v, nil
}
func (s *Service) Sitemap(ctx context.Context, page int) (string, error) {
	c, e := s.SEO(ctx)
	if e != nil {
		return "", e
	}
	if !c.SEOEnabled || !c.IndexingEnabled {
		return xmlEmpty(), nil
	}
	const n = 49998
	var count int64
	if e = s.db.QueryRowContext(ctx, `SELECT count(*) FROM novel_books WHERE deleted_at IS NULL AND publish_status='published'`).Scan(&count); e != nil {
		return "", e
	}
	pages := int((count + n - 1) / n)
	if pages < 1 {
		pages = 1
	}
	if page < 0 || page > pages {
		return "", sql.ErrNoRows
	}
	if page == 0 && count > n {
		var b strings.Builder
		b.WriteString(xmlHeader + "<sitemapindex xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
		for i := 1; i <= pages; i++ {
			fmt.Fprintf(&b, "<sitemap><loc>%s/reader/seo/sitemap-books-%d.xml</loc></sitemap>\n", html.EscapeString(c.SiteURL), i)
		}
		b.WriteString("</sitemapindex>\n")
		return b.String(), nil
	}
	if page > 0 && count <= n {
		return "", sql.ErrNoRows
	}
	offset := page * n
	if page > 0 {
		offset = (page - 1) * n
	}
	rows, e := s.db.QueryContext(ctx, `SELECT id,updated_at FROM novel_books WHERE deleted_at IS NULL AND publish_status='published' ORDER BY updated_at DESC,id DESC LIMIT $1 OFFSET $2`, n, offset)
	if e != nil {
		return "", e
	}
	defer rows.Close()
	var b strings.Builder
	b.WriteString(xmlHeader + "<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	if page <= 1 {
		fmt.Fprintf(&b, "<url><loc>%s/</loc></url>\n<url><loc>%s/books</loc></url>\n", html.EscapeString(c.SiteURL), html.EscapeString(c.SiteURL))
	}
	for rows.Next() {
		var id int64
		var t time.Time
		if e = rows.Scan(&id, &t); e != nil {
			return "", e
		}
		fmt.Fprintf(&b, "<url><loc>%s/books/%d</loc><lastmod>%s</lastmod></url>\n", html.EscapeString(c.SiteURL), id, t.UTC().Format(time.RFC3339))
	}
	b.WriteString("</urlset>\n")
	return b.String(), rows.Err()
}

const xmlHeader = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"

func xmlEmpty() string {
	return xmlHeader + "<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\"></urlset>\n"
}
