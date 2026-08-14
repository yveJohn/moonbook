//go:build integration

package importtask

import (
	"context"
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/chapters"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"strconv"
	"testing"
	"time"
)

func TestChaptersWriterMinIOIdempotency(t *testing.T) {
	db, c := integrationtest.RequireDB(t)
	m := integrationtest.RequireMinIO(t, db, c)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	suffix := time.Now().UnixNano()
	categoryID, authorID, bookID := suffix, suffix+1, suffix+2
	code := fmt.Sprintf("forum-writer-%d", suffix)
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_categories(id,code,name,kind,source) VALUES($1,$2,$3,'primary','native')`, categoryID, code, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_authors(id,pen_name,normalized_name,status,source) VALUES($1,$2,$2,'active','native')`, authorID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_books(id,primary_category_id,category_code,category_name,book_name,author_id,author_name) VALUES($1,$2,$3,$3,$3,$4,$3)`, bookID, categoryID, code, authorID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		rows, _ := db.QueryContext(cleanup, `SELECT object_key FROM novel_objects WHERE book_id=$1`, bookID)
		if rows != nil {
			for rows.Next() {
				var key string
				if rows.Scan(&key) == nil {
					_ = m.Store.Remove(cleanup, key)
				}
			}
			rows.Close()
		}
		for _, q := range []string{`DELETE FROM novel_object_references WHERE book_id=$1`, `DELETE FROM novel_object_events WHERE object_id IN (SELECT id FROM novel_objects WHERE book_id=$1)`, `DELETE FROM novel_objects WHERE book_id=$1`, `DELETE FROM novel_chapters WHERE book_id=$1`, `DELETE FROM novel_books WHERE id=$1`} {
			_, _ = db.ExecContext(cleanup, q, bookID)
		}
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_authors WHERE id=$1`, authorID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_categories WHERE id=$1`, categoryID)
	})
	service := chapters.NewService(db, objectstore.NewService(db, m.Store))
	writer := ChaptersWriter{Service: service}
	task := Task{TargetBookID: strconv.FormatInt(bookID, 10)}
	parsed := []ParsedChapter{{Title: "第一章 初见", Content: "月色落在窗台"}, {Title: "第二章 远行", Content: "列车驶向远方"}}
	first, err := writer.Import(ctx, task, parsed)
	if err != nil || first != 2 {
		t.Fatalf("first=%d err=%v", first, err)
	}
	second, err := writer.Import(ctx, task, parsed)
	if err != nil || second != 0 {
		t.Fatalf("second=%d err=%v", second, err)
	}
	var count, active int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM novel_chapters WHERE book_id=$1 AND deleted_at IS NULL`, bookID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("chapter count=%d err=%v", count, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM novel_object_references r JOIN novel_objects o ON o.id=r.object_id WHERE r.book_id=$1 AND o.state='active'`, bookID).Scan(&active); err != nil || active != 2 {
		t.Fatalf("active objects=%d err=%v", active, err)
	}
}
