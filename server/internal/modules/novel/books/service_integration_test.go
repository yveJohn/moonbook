package books

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestBookLifecycleWithPostgres(t *testing.T) {
	dsn := os.Getenv("MOONBOOK_BOOK_TEST_DSN")
	if dsn == "" {
		t.Skip("MOONBOOK_BOOK_TEST_DSN is not configured")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	suffix := strings.ReplaceAll(time.Now().UTC().Format("150405.000000000"), ".", "")
	var primaryID, subID, authorID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_categories(code,name,kind) VALUES ($1,'主分类','primary') RETURNING id`, "book-primary-"+suffix).Scan(&primaryID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_categories(code,name,kind) VALUES ($1,'副分类','sub') RETURNING id`, "book-sub-"+suffix).Scan(&subID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_authors(pen_name,normalized_name) VALUES ($1,$1) RETURNING id`, "作者-"+suffix).Scan(&authorID); err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	price := int64(9007199254740993)
	input := Input{CategoryCode: "book-primary-" + suffix, BookName: "书籍-" + suffix, AuthorID: authorID, Description: "简介", Score: "9.25", BookStatus: "serializing", PublishStatus: "draft", SourceType: "manual", Featured: true, FeaturedSort: 2, ChargeMode: "fixed_price", FixedPriceCoin: &price, SubCategoryCodes: []string{"book-sub-" + suffix}, Tags: []string{"群像", "成长"}}
	book, err := service.Create(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if book.ID <= 0 || book.AuthorID != authorID || book.PrimaryCategory.ID != primaryID || len(book.SubCategories) != 1 || book.SubCategories[0].ID != subID || len(book.Tags) != 2 || book.FixedPriceCoin == nil || *book.FixedPriceCoin != price {
		t.Fatalf("created book=%+v", book)
	}
	if _, err := service.Create(ctx, input); apperror.Expose(err).Code != apperror.CodeConflict {
		t.Fatalf("duplicate error=%v", err)
	}
	input.BookName = "书籍已更新-" + suffix
	input.PublishStatus = "published"
	input.ChargeMode = "login_free"
	input.FixedPriceCoin = nil
	input.SubCategoryCodes = nil
	input.Tags = []string{"短篇"}
	book, err = service.Update(ctx, book.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if book.PublishStatus != "published" || book.FixedPriceCoin != nil || len(book.SubCategories) != 0 || len(book.Tags) != 1 {
		t.Fatalf("updated book=%+v", book)
	}
	page, err := service.List(ctx, Filter{Page: 1, PageSize: 10, Keyword: "书籍已更新-" + suffix, PublishStatus: "published"})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != book.ID {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	secondInput := input
	secondInput.BookName = "书籍批量装载-" + suffix
	secondInput.SubCategoryCodes = []string{"book-sub-" + suffix}
	secondInput.Tags = []string{"批量"}
	second, err := service.Create(ctx, secondInput)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Delete(context.Background(), second.ID)
	page, err = service.List(ctx, Filter{Page: 1, PageSize: 10, PublishStatus: "published"})
	var foundSecond bool
	for _, item := range page.Items {
		if item.ID == second.ID {
			foundSecond = len(item.SubCategories) == 1 && len(item.Tags) == 1 && item.Tags[0] == "批量"
		}
	}
	if err != nil || !foundSecond {
		t.Fatalf("batched list did not load second relations: page=%+v err=%v", page, err)
	}
	if err := service.Delete(ctx, book.ID); err != nil {
		t.Fatal(err)
	}
	page, err = service.List(ctx, Filter{Page: 1, PageSize: 10, Keyword: "书籍已更新-" + suffix})
	if err != nil || page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("deleted page=%+v err=%v", page, err)
	}
	if err := service.Delete(ctx, book.ID); apperror.Expose(err).Code != apperror.CodeNotFound {
		t.Fatalf("second delete error=%v", err)
	}
}
