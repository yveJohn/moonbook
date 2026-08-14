package chapters

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestChapterLifecycleWithPostgresAndMinIO(t *testing.T) {
	dsn := os.Getenv("MOONBOOK_CHAPTER_TEST_DSN")
	endpoint := os.Getenv("MOONBOOK_CHAPTER_TEST_MINIO_ENDPOINT")
	accessKey := os.Getenv("MOONBOOK_CHAPTER_TEST_MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MOONBOOK_CHAPTER_TEST_MINIO_SECRET_KEY")
	if dsn == "" || endpoint == "" || accessKey == "" || secretKey == "" {
		t.Skip("Moonbook chapter integration test environment is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, "")})
	if err != nil {
		t.Fatal(err)
	}
	bucket := fmt.Sprintf("moonbook-chapter-test-%d", time.Now().UnixNano())
	if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		t.Fatal(err)
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
	})
	store, err := objectstore.NewMinIOStore(objectstore.MinIOConfig{Endpoint: endpoint, AccessKey: accessKey, SecretKey: secretKey, Bucket: bucket})
	if err != nil {
		t.Fatal(err)
	}
	objects := objectstore.NewService(db, store)
	service := NewService(db, objects)
	suffix := time.Now().UnixNano()
	categoryID, authorID, bookID := suffix, suffix+1, suffix+2
	code := fmt.Sprintf("chapter-test-%d", suffix)
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_categories(id,code,name,kind,source) VALUES ($1,$2,$3,'primary','native')`, categoryID, code, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_authors(id,pen_name,normalized_name,status,source) VALUES ($1,$2,$2,'active','native')`, authorID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_books(id,primary_category_id,category_code,category_name,book_name,author_id,author_name) VALUES ($1,$2,$3,$3,$3,$4,$3)`, bookID, categoryID, code, authorID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		rows, _ := db.QueryContext(cleanupCtx, `SELECT object_key FROM novel_objects WHERE book_id=$1`, bookID)
		var keys []string
		if rows != nil {
			for rows.Next() {
				var key string
				if rows.Scan(&key) == nil {
					keys = append(keys, key)
				}
			}
			rows.Close()
		}
		for _, key := range keys {
			_ = store.Remove(cleanupCtx, key)
		}
		for _, query := range []string{
			`DELETE FROM novel_object_references WHERE book_id=$1`,
			`DELETE FROM novel_object_events WHERE object_id IN (SELECT id FROM novel_objects WHERE book_id=$1)`,
			`DELETE FROM novel_objects WHERE book_id=$1`,
			`DELETE FROM novel_chapters WHERE book_id=$1`,
			`DELETE FROM novel_books WHERE id=$1`,
		} {
			_, _ = db.ExecContext(cleanupCtx, query, bookID)
		}
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM novel_authors WHERE id=$1`, authorID)
		_, _ = db.ExecContext(cleanupCtx, `DELETE FROM novel_categories WHERE id=$1`, categoryID)
	})

	firstText := "第一章\n月色 good\n"
	first, err := service.Create(ctx, Input{BookID: bookID, ChapterName: "第一章", IsVIP: false, BookPriceCoin: 0, ChapterStatus: "enabled", AICleanStatus: "pending", Content: &firstText})
	if err != nil {
		t.Fatal(err)
	}
	if first.ChapterNo != 0 || first.WordCount != 9 || first.SourceType != "manual" {
		t.Fatalf("created chapter=%+v", first)
	}
	content, err := service.ReadContent(ctx, first.ID)
	if err != nil || content.Text != firstText || content.Version != 1 {
		t.Fatalf("first content=%+v err=%v", content, err)
	}
	assertBookStats(t, ctx, db, bookID, 9, first.ID, "第一章")

	secondText := "第二版正文已经更长"
	chapterNo := 2
	updated, err := service.Update(ctx, first.ID, Input{BookID: bookID, ChapterNo: &chapterNo, ChapterName: "第二章", IsVIP: true, BookPriceCoin: 9007199254740993, ChapterStatus: "enabled", AICleanStatus: "expired", Content: &secondText})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ChapterNo != 2 || updated.BookPriceCoin != 9007199254740993 {
		t.Fatalf("updated chapter=%+v", updated)
	}
	content, err = service.ReadContent(ctx, first.ID)
	if err != nil || content.Text != secondText || content.Version != 2 {
		t.Fatalf("second content=%+v err=%v", content, err)
	}
	var active, orphaned int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE state='active'),count(*) FILTER (WHERE state='orphaned') FROM novel_objects WHERE book_id=$1 AND owner_id=$2`, bookID, first.ID).Scan(&active, &orphaned); err != nil || active != 1 || orphaned != 1 {
		t.Fatalf("object states active=%d orphaned=%d err=%v", active, orphaned, err)
	}
	assertBookStats(t, ctx, db, bookID, countWords(secondText), first.ID, "第二章")

	if err := service.Delete(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Get(ctx, first.ID); err == nil {
		t.Fatal("soft-deleted chapter is still visible")
	}
	if _, err := service.ReadContent(ctx, first.ID); err == nil {
		t.Fatal("soft-deleted chapter content is still visible")
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE state='active'),count(*) FILTER (WHERE state='orphaned') FROM novel_objects WHERE book_id=$1 AND owner_id=$2`, bookID, first.ID).Scan(&active, &orphaned); err != nil || active != 0 || orphaned != 2 {
		t.Fatalf("deleted object states active=%d orphaned=%d err=%v", active, orphaned, err)
	}
	assertBookStats(t, ctx, db, bookID, 0, 0, "")
}

func assertBookStats(t *testing.T, ctx context.Context, db *sql.DB, bookID int64, words int, chapterID int64, chapterName string) {
	t.Helper()
	var gotWords int
	var gotID sql.NullInt64
	var gotName sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT word_count,last_chapter_id,last_chapter_name FROM novel_books WHERE id=$1`, bookID).Scan(&gotWords, &gotID, &gotName); err != nil {
		t.Fatal(err)
	}
	if gotWords != words || gotID.Valid != (chapterID != 0) || (gotID.Valid && gotID.Int64 != chapterID) || gotName.Valid != (chapterName != "") || (gotName.Valid && gotName.String != chapterName) {
		t.Fatalf("book stats words=%d id=%v name=%v", gotWords, gotID, gotName)
	}
}
