//go:build integration

package provider

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

func TestPurchaseProviderReturnsLiveTargetsAndRequiresTransactionForLocks(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	categoryID, authorID := base, base+1
	publishedBookID, draftBookID := base+2, base+3
	enabledChapterID, disabledChapterID := base+4, base+5
	code := fmt.Sprintf("purchase-provider-%d", base)
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_categories(id,code,name,kind,source) VALUES($1,$2,$2,'primary','native')`, categoryID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_authors(id,pen_name,normalized_name,status,source) VALUES($1,$2,$2,'active','native')`, authorID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_books(id,primary_category_id,category_code,category_name,book_name,author_id,author_name,publish_status,source_type) VALUES($1,$2,$3,$3,'已发布作品',$4,$3,'published','manual'),($5,$2,$3,$3,'草稿作品',$4,$3,'draft','manual')`, publishedBookID, categoryID, code, authorID, draftBookID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_chapters(id,book_id,chapter_no,chapter_name,word_count,chapter_status,ai_clean_status,source_type) VALUES($1,$2,1,'启用章节',1201,'enabled','pending','manual'),($3,$2,2,'禁用章节',800,'disabled','pending','manual')`, enabledChapterID, publishedBookID, disabledChapterID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		q := context.Background()
		_, _ = db.ExecContext(q, `DELETE FROM novel_chapters WHERE id IN ($1,$2)`, enabledChapterID, disabledChapterID)
		_, _ = db.ExecContext(q, `DELETE FROM novel_books WHERE id IN ($1,$2)`, publishedBookID, draftBookID)
		_, _ = db.ExecContext(q, `DELETE FROM novel_authors WHERE id=$1`, authorID)
		_, _ = db.ExecContext(q, `DELETE FROM novel_categories WHERE id=$1`, categoryID)
	})

	provider := NewPurchase(db)
	book, err := provider.ProductTarget(ctx, "book", publishedBookID)
	if err != nil || !book.Enabled || book.BookID != publishedBookID || book.Name != "已发布作品" {
		t.Fatalf("book=%+v err=%v", book, err)
	}
	draft, err := provider.ProductTarget(ctx, "book", draftBookID)
	if err != nil || draft.Enabled {
		t.Fatalf("draft=%+v err=%v", draft, err)
	}
	chapter, err := provider.ProductTarget(ctx, "chapter", enabledChapterID)
	if err != nil || !chapter.Enabled || chapter.BookID != publishedBookID {
		t.Fatalf("chapter=%+v err=%v", chapter, err)
	}
	disabled, err := provider.ProductTarget(ctx, "chapter", disabledChapterID)
	if err != nil || disabled.Enabled {
		t.Fatalf("disabled=%+v err=%v", disabled, err)
	}
	if _, err := provider.ProductTarget(ctx, "book", base+999); !errors.Is(err, novelcontract.ErrBookNotFound) {
		t.Fatalf("missing book err=%v", err)
	}
	if _, err := provider.ProductTarget(ctx, "chapter", base+999); !errors.Is(err, novelcontract.ErrChapterNotFound) {
		t.Fatalf("missing chapter err=%v", err)
	}
	if _, err := provider.LockPurchaseSnapshot(ctx, "chapter", enabledChapterID); !errors.Is(err, novelcontract.ErrUnavailable) || !errors.Is(novelcontract.Cause(err), transaction.ErrNoTransaction) {
		t.Fatalf("lock without transaction err=%v", err)
	}
	if err := transaction.New(db).Within(ctx, func(txCtx context.Context) error {
		snapshot, err := provider.LockPurchaseSnapshot(txCtx, "chapter", enabledChapterID)
		if err != nil || !snapshot.Enabled || snapshot.WordCount != 1201 || snapshot.Name != "启用章节" {
			t.Fatalf("snapshot=%+v err=%v", snapshot, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
