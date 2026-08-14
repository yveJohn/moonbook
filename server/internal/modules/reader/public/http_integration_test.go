//go:build integration

package public

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/catalog"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"github.com/gin-gonic/gin"
)

type readerFaultBlobStore struct {
	objectstore.BlobStore
	mode string
}

func (store readerFaultBlobStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	switch store.mode {
	case "missing":
		return nil, errors.New("open MinIO object: NoSuchKey at internal endpoint")
	case "corrupt":
		return io.NopCloser(strings.NewReader("corrupt chapter body")), nil
	case "timeout":
		return nil, context.DeadlineExceeded
	default:
		return store.BlobStore.Get(ctx, key)
	}
}

func TestReaderPublicHTTPContractWithRealDependencies(t *testing.T) {
	db, cfg := integrationtest.RequireDB(t)
	minioTest := integrationtest.RequireMinIO(t, db, cfg)
	redis := integrationtest.RequireRedis(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	base := time.Now().UnixNano()
	reader, category, subCategory, author, book, chapter, emptyBook := base, base+1, base+2, base+3, base+4, base+5, base+6
	code := integrationtest.Prefix()
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'x','enabled')`, reader, code+"-reader"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_categories(id,code,name,kind,source) VALUES($1,$2,$2,'primary','native')`, category, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_categories(id,code,name,kind,source) VALUES($1,$2,$2,'sub','native')`, subCategory, code+"-sub"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_authors(id,pen_name,normalized_name,status,source) VALUES($1,$2,$2,'active','native')`, author, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_books(id,primary_category_id,category_code,category_name,book_name,author_id,author_name,publish_status,book_status,source_type,charge_mode,featured,featured_note) VALUES($1,$2,$3,$3,$3,$4,$3,'published','serializing','manual','login_free',true,'HTTP 集成推荐')`, book, category, code, author); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_books(id,primary_category_id,category_code,category_name,book_name,author_id,author_name,publish_status,book_status,source_type,charge_mode) VALUES($1,$2,$3,$3,$4,$5,$3,'published','serializing','manual','login_free')`, emptyBook, category, code, code+"-empty", author); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_book_sub_categories(book_id,category_id,category_code,category_name,sort) VALUES($1,$2,$3,$3,0)`, book, subCategory, code+"-sub"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_chapters(id,book_id,chapter_no,chapter_name,word_count,chapter_status,ai_clean_status,source_type) VALUES($1,$2,1,'HTTP 集成章节',12,'enabled','pending','manual')`, chapter, book); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE novel_books SET last_chapter_id=$2,last_chapter_name='HTTP 集成章节',last_chapter_updated_at=(SELECT updated_at FROM novel_chapters WHERE id=$2) WHERE id=$1`, book, chapter); err != nil {
		t.Fatal(err)
	}
	objects := objectstore.NewService(db, minioTest.Store)
	content := []byte("真实 HTTP MinIO 正文\n")
	obj, err := objects.UploadVerified(ctx, objectstore.Target{Kind: objectstore.KindChapterContent, BookID: book, OwnerID: chapter}, content, "text/plain; charset=utf-8")
	if err != nil {
		t.Fatal(err)
	}
	if err := objects.Activate(ctx, obj.ID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_object_references WHERE book_id=$1`, book)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_object_events WHERE object_id IN (SELECT id FROM novel_objects WHERE book_id=$1)`, book)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_objects WHERE book_id=$1`, book)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_chapters WHERE id=$1`, chapter)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_book_sub_categories WHERE book_id=$1`, book)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_books WHERE id IN ($1,$2)`, book, emptyBook)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_authors WHERE id=$1`, author)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_categories WHERE id IN ($1,$2)`, category, subCategory)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_sessions WHERE reader_id=$1`, reader)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_accounts WHERE id=$1`, reader)
	})
	auth := readerauth.NewService(readerauth.SQLRepository{DB: db}, readerauth.RedisRateLimiter{Client: redis, Prefix: "moonbook:reader:http-it:rate:"}, readerauth.TokenConfig{Secret: []byte("reader-http-integration-secret-32"), TTL: time.Hour, Issuer: "moonbook-reader-http-it"})
	router := gin.New()
	RegisterRoutes(router.Group("/"), db, objects, auth, catalog.NewService(catalog.SQLRepository{DB: db}))
	token, err := auth.CreateToken(ctx, reader)
	if err != nil {
		t.Fatal(err)
	}
	requestRaw := func(method, path string, authenticated bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		}
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		return resp
	}
	request := func(method, path string, authenticated bool) map[string]any {
		resp := requestRaw(method, path, authenticated)
		if resp.Code != http.StatusOK {
			t.Fatalf("%s %s status=%d body=%s", method, path, resp.Code, resp.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s %s decode: %v", method, path, err)
		}
		return body
	}
	containsBook := func(items []any) bool {
		for _, item := range items {
			if value, ok := item.(map[string]any); ok && value["bookId"] == fmt.Sprint(book) {
				return true
			}
		}
		return false
	}
	containsCategory := func(items []any, want string) bool {
		for _, item := range items {
			if value, ok := item.(map[string]any); ok && value["code"] == want {
				return true
			}
		}
		return false
	}
	assertDateTime := func(label string, value any) {
		t.Helper()
		date, ok := value.(string)
		if !ok {
			t.Fatalf("%s date=%v", label, value)
		}
		if _, err := time.Parse("2006-01-02 15:04:05", date); err != nil {
			t.Fatalf("%s date=%q: %v", label, date, err)
		}
	}

	featured := request(http.MethodGet, "/reader/books/featured", false)
	featuredRows, ok := featured["data"].([]any)
	if !ok || !containsBook(featuredRows) {
		t.Fatalf("featured response=%v", featured)
	}
	random := request(http.MethodGet, "/reader/books/random", false)
	randomRows, ok := random["data"].([]any)
	if !ok || !containsBook(randomRows) {
		t.Fatalf("random response=%v", random)
	}
	books := request(http.MethodGet, fmt.Sprintf("/reader/books?keyword=%s&subCategoryCode=%s-sub&pageNum=1&pageSize=10", code, code), false)
	rows, ok := books["rows"].([]any)
	if !ok || len(rows) != 1 || rows[0].(map[string]any)["bookId"] != fmt.Sprint(book) {
		t.Fatalf("books response=%v", books)
	}
	emptyBooks := request(http.MethodGet, fmt.Sprintf("/reader/books?keyword=%s-missing&pageNum=0&pageSize=0", code), false)
	emptyRows, ok := emptyBooks["rows"].([]any)
	if !ok || len(emptyRows) != 0 || emptyBooks["total"] != float64(0) {
		t.Fatalf("empty books response=%v", emptyBooks)
	}
	categories := request(http.MethodGet, "/reader/books/categories", false)
	categoryRows, ok := categories["data"].([]any)
	if !ok || !containsCategory(categoryRows, code) {
		t.Fatalf("categories response=%v", categories)
	}
	subCategories := request(http.MethodGet, "/reader/books/sub-categories", false)
	subCategoryRows, ok := subCategories["data"].([]any)
	if !ok || !containsCategory(subCategoryRows, code+"-sub") {
		t.Fatalf("sub-categories response=%v", subCategories)
	}
	detail := request(http.MethodGet, fmt.Sprintf("/reader/books/%d", book), true)
	detailData, ok := detail["data"].(map[string]any)
	status, statusOK := detailData["productStatus"].(map[string]any)
	if !ok || detailData["bookId"] != fmt.Sprint(book) || detailData["lastChapterId"] != fmt.Sprint(chapter) || !statusOK || status["readable"] != true || status["accessReason"] != "login_free" {
		t.Fatalf("book detail response=%v", detail)
	}
	assertDateTime("book last chapter", detailData["lastChapterUpdateTime"])
	chapters := request(http.MethodGet, fmt.Sprintf("/reader/books/%d/chapters", book), true)
	chapterRows := chapters["data"].([]any)
	if len(chapterRows) != 1 || chapterRows[0].(map[string]any)["chapterId"] != fmt.Sprint(chapter) {
		t.Fatalf("chapters response=%v", chapters)
	}
	assertDateTime("chapter update", chapterRows[0].(map[string]any)["updateTime"])
	emptyChapters := request(http.MethodGet, fmt.Sprintf("/reader/books/%d/chapters", emptyBook), false)
	emptyChapterRows, ok := emptyChapters["data"].([]any)
	if !ok || len(emptyChapterRows) != 0 {
		t.Fatalf("empty chapters response=%v", emptyChapters)
	}
	anonymousChapter := request(http.MethodGet, fmt.Sprintf("/reader/chapters/%d", chapter), false)
	if anonymousChapter["code"] != float64(46101) || anonymousChapter["msg"] != "请先登录后阅读" {
		t.Fatalf("anonymous chapter response=%v", anonymousChapter)
	}
	chapterBody := request(http.MethodGet, fmt.Sprintf("/reader/chapters/%d", chapter), true)
	data, ok := chapterBody["data"].(map[string]any)
	if !ok || data["content"] != string(content) {
		t.Fatalf("chapter response=%v", chapterBody)
	}
	for _, mode := range []string{"missing", "corrupt", "timeout"} {
		t.Run("chapter content "+mode, func(t *testing.T) {
			faultRouter := gin.New()
			faultObjects := objectstore.NewService(db, readerFaultBlobStore{BlobStore: minioTest.Store, mode: mode})
			RegisterRoutes(faultRouter.Group("/"), db, faultObjects, auth, catalog.NewService(catalog.SQLRepository{DB: db}))
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/reader/chapters/%d", chapter), nil)
			req.Header.Set("Authorization", "Bearer "+token.AccessToken)
			resp := httptest.NewRecorder()
			faultRouter.ServeHTTP(resp, req)
			var body map[string]any
			if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			data, hasData := body["data"]
			if resp.Code != http.StatusOK || body["code"] != float64(500) || body["msg"] != "读取章节正文失败" || !hasData || data != nil {
				t.Fatalf("mode=%s status=%d body=%s", mode, resp.Code, resp.Body.String())
			}
			for _, secret := range []string{"NoSuchKey", "internal endpoint", "chapters/"} {
				if strings.Contains(resp.Body.String(), secret) {
					t.Fatalf("mode=%s leaked %q: %s", mode, secret, resp.Body.String())
				}
			}
		})
	}

	seo := request(http.MethodGet, "/reader/seo/config", false)
	seoData, ok := seo["data"].(map[string]any)
	if !ok || seoData["id"] != "1" || seoData["siteUrl"] == "" {
		t.Fatalf("seo response=%v", seo)
	}
	robots := requestRaw(http.MethodGet, "/reader/seo/robots.txt", false)
	if robots.Code != http.StatusOK || !strings.HasPrefix(robots.Header().Get("Content-Type"), "text/plain") || !strings.Contains(robots.Body.String(), "User-agent: *") {
		t.Fatalf("robots status=%d content-type=%q body=%q", robots.Code, robots.Header().Get("Content-Type"), robots.Body.String())
	}
	sitemap := requestRaw(http.MethodGet, "/reader/seo/sitemap.xml", false)
	if sitemap.Code != http.StatusOK || !strings.HasPrefix(sitemap.Header().Get("Content-Type"), "application/xml") || !strings.Contains(sitemap.Body.String(), fmt.Sprintf("/books/%d", book)) {
		t.Fatalf("sitemap status=%d content-type=%q body=%q", sitemap.Code, sitemap.Header().Get("Content-Type"), sitemap.Body.String())
	}
	for _, path := range []string{"/reader/seo/sitemap-books-0.xml", "/reader/seo/sitemap-books-1.xml"} {
		resp := requestRaw(http.MethodGet, path, false)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("%s status=%d body=%q", path, resp.Code, resp.Body.String())
		}
	}
}
