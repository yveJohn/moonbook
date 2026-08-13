package metadata

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

func TestMetadataLifecycleWithPostgres(t *testing.T) {
	dsn := os.Getenv("MOONBOOK_NOVEL_TEST_DSN")
	if dsn == "" {
		t.Skip("MOONBOOK_NOVEL_TEST_DSN is not configured")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	service := NewService(db)
	suffix := strings.ReplaceAll(time.Now().UTC().Format("150405.000000000"), ".", "")

	category, err := service.CreateCategory(ctx, CategoryInput{
		Code: "integration-" + suffix, Name: "集成分类", Kind: CategoryKindSub, Sort: 7, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if category.ID <= 0 || category.Code != "integration-"+suffix || category.Source != "native" {
		t.Fatalf("created category = %+v", category)
	}
	category, err = service.UpdateCategory(ctx, category.ID, CategoryInput{
		Code: category.Code, Name: "集成分类已更新", Kind: CategoryKindSub, Sort: 3, Enabled: false,
	})
	if err != nil || category.Name != "集成分类已更新" || category.Enabled || category.Sort != 3 {
		t.Fatalf("updated category = %+v, err=%v", category, err)
	}
	categories, err := service.ListCategories(ctx, ListFilter{Page: 1, PageSize: 10, Keyword: category.Code})
	if err != nil || categories.Total != 1 || len(categories.Items) != 1 || categories.Items[0].ID != category.ID {
		t.Fatalf("category page = %+v, err=%v", categories, err)
	}
	if err := service.DeleteCategory(ctx, category.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteCategory(ctx, category.ID); apperror.Expose(err).Code != apperror.CodeNotFound {
		t.Fatalf("second category delete error = %v", err)
	}
	recreatedCategory, err := service.CreateCategory(ctx, CategoryInput{
		Code: category.Code, Name: "软删除后重建", Kind: CategoryKindSub, Enabled: true,
	})
	if err != nil {
		t.Fatalf("recreate soft-deleted category: %v", err)
	}
	if recreatedCategory.ID == category.ID {
		t.Fatalf("recreated category reused deleted id %d", category.ID)
	}
	if err := service.DeleteCategory(ctx, recreatedCategory.ID); err != nil {
		t.Fatal(err)
	}

	author, err := service.CreateAuthor(ctx, AuthorInput{PenName: "集成 作者 " + suffix, Status: AuthorStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	if author.ID <= 0 || author.NormalizedName != "集成作者"+suffix || author.Source != "native" {
		t.Fatalf("created author = %+v", author)
	}
	author, err = service.UpdateAuthor(ctx, author.ID, AuthorInput{PenName: "集成作者 " + suffix, Status: AuthorStatusBlocked})
	if err != nil || author.Status != AuthorStatusBlocked {
		t.Fatalf("updated author = %+v, err=%v", author, err)
	}
	authors, err := service.ListAuthors(ctx, ListFilter{Page: 1, PageSize: 10, Keyword: suffix, Status: AuthorStatusBlocked})
	if err != nil || authors.Total != 1 || len(authors.Items) != 1 || authors.Items[0].ID != author.ID {
		t.Fatalf("author page = %+v, err=%v", authors, err)
	}
	if err := service.DeleteAuthor(ctx, author.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteAuthor(ctx, author.ID); apperror.Expose(err).Code != apperror.CodeNotFound {
		t.Fatalf("second author delete error = %v", err)
	}
}
