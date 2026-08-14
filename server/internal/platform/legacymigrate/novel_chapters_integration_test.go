package legacymigrate

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestNovelChapterMigrationWithMySQLPostgresAndMinIO(t *testing.T) {
	mysqlDSN := os.Getenv("MOONBOOK_LEGACY_CHAPTER_TEST_DSN")
	postgresDSN := os.Getenv("MOONBOOK_MIGRATION_TEST_DSN")
	endpoint := os.Getenv("MOONBOOK_OBJECT_TEST_MINIO_ENDPOINT")
	accessKey := os.Getenv("MOONBOOK_OBJECT_TEST_MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MOONBOOK_OBJECT_TEST_MINIO_SECRET_KEY")
	if mysqlDSN == "" || postgresDSN == "" || endpoint == "" || accessKey == "" || secretKey == "" {
		t.Skip("chapter migration integration environment is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	source, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	target, err := sql.Open("pgx", postgresDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, "")})
	if err != nil {
		t.Fatal(err)
	}
	bucket := fmt.Sprintf("moonbook-chapter-migration-%d", time.Now().UnixNano())
	if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		t.Fatal(err)
	}
	store, err := objectstore.NewMinIOStore(objectstore.MinIOConfig{Endpoint: endpoint, AccessKey: accessKey, SecretKey: secretKey, Bucket: bucket})
	if err != nil {
		t.Fatal(err)
	}
	objects := objectstore.NewService(target, store)
	const categoryID int64 = 9007199254741001
	const authorID int64 = 9007199254741002
	const bookID int64 = 9007199254741000
	code := fmt.Sprintf("chapter-migration-%d", time.Now().UnixNano())
	if _, err := target.ExecContext(ctx, `INSERT INTO novel_categories(id,code,name,kind,source) VALUES ($1,$2,$2,'primary','native')`, categoryID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := target.ExecContext(ctx, `INSERT INTO novel_authors(id,pen_name,normalized_name,status,source) VALUES ($1,$2,$2,'active','native')`, authorID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := target.ExecContext(ctx, `INSERT INTO novel_books(id,primary_category_id,category_code,category_name,book_name,author_id,author_name) VALUES ($1,$2,$3,$3,$3,$4,$3)`, bookID, categoryID, code, authorID); err != nil {
		t.Fatal(err)
	}
	migrations := []string{
		fmt.Sprintf("novel-chapters-integration-%d-a", time.Now().UnixNano()),
		fmt.Sprintf("novel-chapters-integration-%d-b", time.Now().UnixNano()),
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		for object := range client.ListObjects(cleanupCtx, bucket, minio.ListObjectsOptions{Recursive: true}) {
			if object.Err == nil {
				_ = client.RemoveObject(cleanupCtx, bucket, object.Key, minio.RemoveObjectOptions{})
			}
		}
		_ = client.RemoveBucket(cleanupCtx, bucket)
		_, _ = target.ExecContext(cleanupCtx, `DELETE FROM novel_object_references WHERE book_id=$1`, bookID)
		_, _ = target.ExecContext(cleanupCtx, `DELETE FROM novel_object_events WHERE object_id IN (SELECT id FROM novel_objects WHERE book_id=$1)`, bookID)
		_, _ = target.ExecContext(cleanupCtx, `DELETE FROM novel_objects WHERE book_id=$1`, bookID)
		_, _ = target.ExecContext(cleanupCtx, `DELETE FROM novel_chapters WHERE book_id=$1`, bookID)
		_, _ = target.ExecContext(cleanupCtx, `DELETE FROM novel_books WHERE id=$1`, bookID)
		_, _ = target.ExecContext(cleanupCtx, `DELETE FROM novel_authors WHERE id=$1`, authorID)
		_, _ = target.ExecContext(cleanupCtx, `DELETE FROM novel_categories WHERE id=$1`, categoryID)
		for _, migration := range migrations {
			_, _ = target.ExecContext(cleanupCtx, `DELETE FROM migration_errors WHERE migration_name=$1`, migration)
			_, _ = target.ExecContext(cleanupCtx, `DELETE FROM migration_checkpoints WHERE migration_name=$1`, migration)
		}
	})
	run := func(migration string) {
		t.Helper()
		runner, err := NewRunner(source, target, migration, 2)
		if err != nil {
			t.Fatal(err)
		}
		runner.verifySource = func(context.Context, *sql.DB) error { return nil }
		if err := runner.Run(ctx, NovelChaptersStage{Objects: objects}); err != nil {
			t.Fatal(err)
		}
	}
	run(migrations[0])
	run(migrations[1])
	assertCount := func(query string, args []any, want int) {
		t.Helper()
		var got int
		if err := target.QueryRowContext(ctx, query, args...).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("query %q count=%d want=%d", query, got, want)
		}
	}
	assertCount(`SELECT count(*) FROM novel_chapters WHERE id IN (9007199254741010,9007199254741011)`, nil, 2)
	assertCount(`SELECT count(*) FROM novel_chapters WHERE id=9007199254741010 AND word_count=9 AND chapter_status='enabled' AND source_type='legacy'`, nil, 1)
	assertCount(`SELECT count(*) FROM novel_chapters WHERE id=9007199254741011 AND book_price_coin=9007199254740993 AND chapter_status='disabled' AND ai_clean_status='expired'`, nil, 1)
	assertCount(`SELECT count(*) FROM novel_objects WHERE book_id=$1 AND object_kind='chapter_content'`, []any{bookID}, 2)
	assertCount(`SELECT count(*) FROM novel_object_references WHERE book_id=$1 AND object_kind='chapter_content'`, []any{bookID}, 2)
	for code, want := range map[string]int{
		"WORD_COUNT_RECALCULATED":   2,
		"INVALID_CHAPTER_STATUS":    1,
		"BOOK_NOT_FOUND":            1,
		"CHAPTER_CONTENT_NOT_FOUND": 1,
	} {
		assertCount(`SELECT count(*) FROM migration_errors WHERE migration_name=$1 AND error_code=$2`, []any{migrations[0], code}, want)
	}
	data, active, err := objects.ReadActive(ctx, objectstore.Target{Kind: objectstore.KindChapterContent, BookID: bookID, OwnerID: 9007199254741010})
	if err != nil || string(data) != "第一章 正文 test" || active.Version != 1 {
		t.Fatalf("active=%+v data=%q err=%v", active, data, err)
	}
	var generatedID int64
	if err := target.QueryRowContext(ctx, `INSERT INTO novel_chapters(book_id,chapter_no,chapter_name) VALUES ($1,99,'序列验证') RETURNING id`, bookID).Scan(&generatedID); err != nil {
		t.Fatal(err)
	}
	if generatedID <= 9007199254741011 {
		t.Fatalf("novel chapter identity did not advance: %d", generatedID)
	}
	if _, err := target.ExecContext(ctx, `DELETE FROM novel_chapters WHERE id=$1`, generatedID); err != nil {
		t.Fatal(err)
	}
}
