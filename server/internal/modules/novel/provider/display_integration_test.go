//go:build integration

package provider

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
)

func TestDisplayProviderKeepsOrderAndDistinguishesUnavailableRows(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	categoryID, authorID := base, base+1
	publishedBook, draftBook, deletedBook := base+2, base+3, base+4
	enabledChapter, deletedChapter := base+5, base+6
	missingID := base + 7
	code := fmt.Sprintf("display-it-%d", base)

	if _, err := db.ExecContext(ctx, `INSERT INTO novel_categories(id,code,name,kind,source) VALUES($1,$2,$2,'primary','native')`, categoryID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_authors(id,pen_name,normalized_name,status,source) VALUES($1,$2,$2,'active','native')`, authorID, code); err != nil {
		t.Fatal(err)
	}
	for _, book := range []struct {
		id, suffix int64
		status     string
	}{
		{publishedBook, 1, "published"},
		{draftBook, 2, "draft"},
		{deletedBook, 3, "published"},
	} {
		name := fmt.Sprintf("%s-%d", code, book.suffix)
		if _, err := db.ExecContext(ctx, `INSERT INTO novel_books(id,primary_category_id,category_code,category_name,book_name,author_id,author_name,publish_status) VALUES($1,$2,$3,$3,$4,$5,$3,$6)`, book.id, categoryID, code, name, authorID, book.status); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `UPDATE novel_books SET deleted_at=now() WHERE id=$1`, deletedBook); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_chapters(id,book_id,chapter_no,chapter_name,chapter_status,ai_clean_status,source_type) VALUES($1,$2,1,'可见章节','enabled','pending','manual'),($3,$2,2,'删除章节','enabled','pending','manual')`, enabledChapter, publishedBook, deletedChapter); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE novel_chapters SET deleted_at=now() WHERE id=$1`, deletedChapter); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_chapters WHERE id IN ($1,$2)`, enabledChapter, deletedChapter)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_books WHERE id IN ($1,$2,$3)`, publishedBook, draftBook, deletedBook)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_authors WHERE id=$1`, authorID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_categories WHERE id=$1`, categoryID)
	})

	request := novelcontract.DisplayRequest{
		BookIDs:    []int64{draftBook, publishedBook, missingID, deletedBook, publishedBook},
		ChapterIDs: []int64{deletedChapter, missingID, enabledChapter, enabledChapter},
	}
	batch, err := NewDisplay(db).BatchDisplay(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Books) != len(request.BookIDs) || len(batch.Chapters) != len(request.ChapterIDs) {
		t.Fatalf("batch lengths books=%d chapters=%d", len(batch.Books), len(batch.Chapters))
	}
	for index, id := range request.BookIDs {
		if batch.Books[index].ID != id {
			t.Fatalf("book order[%d]=%d want %d", index, batch.Books[index].ID, id)
		}
	}
	if !batch.Books[0].Found || batch.Books[0].Published || !batch.Books[1].Published || batch.Books[2].Found || !batch.Books[3].Found || batch.Books[3].Published || !batch.Books[4].Published {
		t.Fatalf("unexpected book states: %+v", batch.Books)
	}
	for index, id := range request.ChapterIDs {
		if batch.Chapters[index].ID != id {
			t.Fatalf("chapter order[%d]=%d want %d", index, batch.Chapters[index].ID, id)
		}
	}
	if !batch.Chapters[0].Found || batch.Chapters[0].Enabled || batch.Chapters[1].Found || !batch.Chapters[2].Enabled || !batch.Chapters[3].Enabled {
		t.Fatalf("unexpected chapter states: %+v", batch.Chapters)
	}
}
