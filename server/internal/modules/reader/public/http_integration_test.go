//go:build integration

package public

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/catalog"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"github.com/gin-gonic/gin"
)

func TestReaderPublicHTTPContractWithRealDependencies(t *testing.T) {
	db, cfg := integrationtest.RequireDB(t)
	minioTest := integrationtest.RequireMinIO(t, db, cfg)
	redis := integrationtest.RequireRedis(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	base := time.Now().UnixNano()
	reader, category, author, book, chapter := base, base+1, base+2, base+3, base+4
	code := integrationtest.Prefix()
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'x','enabled')`, reader, code+"-reader"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_categories(id,code,name,kind,source) VALUES($1,$2,$2,'primary','native')`, category, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_authors(id,pen_name,normalized_name,status,source) VALUES($1,$2,$2,'active','native')`, author, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_books(id,primary_category_id,category_code,category_name,book_name,author_id,author_name,publish_status,book_status,source_type,charge_mode) VALUES($1,$2,$3,$3,$3,$4,$3,'published','serializing','manual','login_free')`, book, category, code, author); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_chapters(id,book_id,chapter_no,chapter_name,word_count,chapter_status,ai_clean_status,source_type) VALUES($1,$2,1,'HTTP 集成章节',12,'enabled','pending','manual')`, chapter, book); err != nil {
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
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_books WHERE id=$1`, book)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_authors WHERE id=$1`, author)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_categories WHERE id=$1`, category)
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
	request := func(method, path string, authenticated bool) map[string]any {
		req := httptest.NewRequest(method, path, nil)
		if authenticated {
			req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		}
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("%s %s status=%d body=%s", method, path, resp.Code, resp.Body.String())
		}
		var body map[string]any
		if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s %s decode: %v", method, path, err)
		}
		return body
	}
	books := request(http.MethodGet, fmt.Sprintf("/reader/books?keyword=%s&pageNum=1&pageSize=10", code), false)
	rows, ok := books["rows"].([]any)
	if !ok || len(rows) != 1 || rows[0].(map[string]any)["bookId"] != fmt.Sprint(book) {
		t.Fatalf("books response=%v", books)
	}
	chapters := request(http.MethodGet, fmt.Sprintf("/reader/books/%d/chapters", book), true)
	chapterRows := chapters["data"].([]any)
	if len(chapterRows) != 1 || chapterRows[0].(map[string]any)["chapterId"] != fmt.Sprint(chapter) {
		t.Fatalf("chapters response=%v", chapters)
	}
	chapterBody := request(http.MethodGet, fmt.Sprintf("/reader/chapters/%d", chapter), true)
	data, ok := chapterBody["data"].(map[string]any)
	if !ok || data["content"] != string(content) {
		t.Fatalf("chapter response=%v", chapterBody)
	}
}
