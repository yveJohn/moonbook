//go:build integration

package public

import (
	"context"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/catalog"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
)

func TestReaderPublicPublishedBookChapterAndMinIO(t *testing.T) {
	db, cfg := integrationtest.RequireDB(t)
	minioTest := integrationtest.RequireMinIO(t, db, cfg)
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
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_chapters(id,book_id,chapter_no,chapter_name,word_count,chapter_status,ai_clean_status,source_type) VALUES($1,$2,1,'第一章',8,'enabled','pending','manual')`, chapter, book); err != nil {
		t.Fatal(err)
	}
	objects := objectstore.NewService(db, minioTest.Store)
	content := []byte("真实 MinIO 正文\n")
	obj, err := objects.UploadVerified(ctx, objectstore.Target{Kind: objectstore.KindChapterContent, BookID: book, OwnerID: chapter}, content, "text/plain; charset=utf-8")
	if err != nil {
		t.Fatalf("upload verified content: %v", err)
	}
	if err := objects.Activate(ctx, obj.ID); err != nil {
		t.Fatalf("activate content: %v", err)
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
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_accounts WHERE id=$1`, reader)
	})
	service := NewService(db, objects, catalog.NewService(catalog.SQLRepository{DB: db}))
	books, total, err := service.List(ctx, code, "", "", 1, 10)
	if err != nil || total != 1 || len(books) != 1 || books[0].ID != book {
		t.Fatalf("published books=%+v total=%d err=%v", books, total, err)
	}
	chapters, err := service.Chapters(ctx, book, &reader)
	if err != nil || len(chapters) != 1 || chapters[0].ID != chapter || !chapters[0].Access.Readable {
		t.Fatalf("chapter access=%+v err=%v", chapters, err)
	}
	got, text, active, err := service.Chapter(ctx, chapter, &reader)
	if err != nil || got.ID != chapter || text != string(content) || active.ID != obj.ID {
		t.Fatalf("chapter=%+v text=%q object=%+v err=%v", got, text, active, err)
	}
}
