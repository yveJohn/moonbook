package legacymigrate

import (
	"context"
	"database/sql"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestNovelBookCoverMigrationWithPostgresMinIOAndHTTP(t *testing.T) {
	postgresDSN := os.Getenv("MOONBOOK_MIGRATION_TEST_DSN")
	endpoint := os.Getenv("MOONBOOK_OBJECT_TEST_MINIO_ENDPOINT")
	accessKey := os.Getenv("MOONBOOK_OBJECT_TEST_MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MOONBOOK_OBJECT_TEST_MINIO_SECRET_KEY")
	bucket := os.Getenv("MOONBOOK_OBJECT_TEST_MINIO_BUCKET")
	if postgresDSN == "" || endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		t.Skip("PostgreSQL and MinIO cover migration integration environment is not configured")
	}
	db, err := sql.Open("pgx", postgresDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := objectstore.NewMinIOStore(objectstore.MinIOConfig{Endpoint: endpoint, AccessKey: accessKey, SecretKey: secretKey, Bucket: bucket})
	if err != nil {
		t.Fatal(err)
	}
	objects := objectstore.NewService(db, store)

	png := []byte("\x89PNG\r\n\x1a\nlegacy-cover-fixture")
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	fixture := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/valid.png" {
			_, _ = writer.Write(png)
			return
		}
		if request.URL.Path == "/invalid" {
			_, _ = writer.Write([]byte("not an image"))
			return
		}
		http.NotFound(writer, request)
	}))
	fixture.Listener = listener
	fixture.Start()
	defer fixture.Close()
	downloader, err := NewCoverDownloader(2*time.Second, []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	suffix := strings.ReplaceAll(time.Now().UTC().Format("150405.000000000"), ".", "")
	var categoryID, authorID, validBookID, invalidBookID int64
	defer func() {
		rows, _ := db.QueryContext(context.Background(), `SELECT object_key FROM novel_objects WHERE book_id IN ($1,$2)`, validBookID, invalidBookID)
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
			_ = store.Remove(context.Background(), key)
		}
		_, _ = db.ExecContext(context.Background(), `DELETE FROM novel_object_references WHERE book_id IN ($1,$2)`, validBookID, invalidBookID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM novel_object_events WHERE object_id IN (SELECT id FROM novel_objects WHERE book_id IN ($1,$2))`, validBookID, invalidBookID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM novel_objects WHERE book_id IN ($1,$2)`, validBookID, invalidBookID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM novel_books WHERE id IN ($1,$2)`, validBookID, invalidBookID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM novel_authors WHERE id=$1`, authorID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM novel_categories WHERE id=$1`, categoryID)
	}()
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_categories(code,name,kind,source) VALUES ($1,'封面迁移分类','primary','native') RETURNING id`, "cover-migration-"+suffix).Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_authors(pen_name,normalized_name,status,source) VALUES ($1,$1,'active','native') RETURNING id`, "封面迁移作者"+suffix).Scan(&authorID); err != nil {
		t.Fatal(err)
	}
	insertBook := func(name, coverURL string) int64 {
		t.Helper()
		var id int64
		err := db.QueryRowContext(ctx, `INSERT INTO novel_books(primary_category_id,category_code,category_name,legacy_cover_url,book_name,author_id,author_name,source_type)
			VALUES ($1,$2,'封面迁移分类',$3,$4,$5,$6,'legacy') RETURNING id`, categoryID, "cover-migration-"+suffix, coverURL, name, authorID, "封面迁移作者"+suffix).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	validBookID = insertBook("有效封面"+suffix, fixture.URL+"/valid.png")
	invalidBookID = insertBook("错误封面"+suffix, fixture.URL+"/invalid")

	stage := NovelBookCoversStage{Objects: objects, Downloader: downloader}
	run := func() BatchResult {
		t.Helper()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		result, err := stage.RunBatch(ctx, nil, tx, strconv.FormatInt(validBookID-1, 10), 10)
		if err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		return result
	}
	first := run()
	if first.Processed != 2 || len(first.Errors) != 1 || first.Errors[0].Code != "INVALID_COVER_TYPE" || first.Errors[0].SourceID != strconv.FormatInt(invalidBookID, 10) {
		t.Fatalf("first result=%+v", first)
	}
	data, active, err := objects.ReadActive(ctx, objectstore.Target{Kind: objectstore.KindBookCover, BookID: validBookID, OwnerID: validBookID, Extension: "png"})
	if err != nil || string(data) != string(png) || active.Version != 1 {
		t.Fatalf("active=%+v data=%q err=%v", active, data, err)
	}
	nativeData := []byte("\x89PNG\r\n\x1a\nnew-native-cover")
	native, err := objects.UploadVerified(ctx, objectstore.Target{Kind: objectstore.KindBookCover, BookID: validBookID, OwnerID: validBookID, Extension: "png"}, nativeData, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if err := objects.Activate(ctx, native.ID); err != nil {
		t.Fatal(err)
	}
	second := run()
	if second.Processed != 2 || len(second.Errors) != 1 {
		t.Fatalf("second result=%+v", second)
	}
	data, active, err = objects.ReadActive(ctx, objectstore.Target{Kind: objectstore.KindBookCover, BookID: validBookID, OwnerID: validBookID, Extension: "png"})
	if err != nil || string(data) != string(nativeData) || active.ID != native.ID {
		t.Fatalf("rerun replaced native cover: active=%+v data=%q err=%v", active, data, err)
	}
	var objectCount, referenceCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM novel_objects WHERE book_id=$1`, validBookID).Scan(&objectCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM novel_object_references WHERE book_id=$1`, validBookID).Scan(&referenceCount); err != nil {
		t.Fatal(err)
	}
	if objectCount != 2 || referenceCount != 1 {
		t.Fatalf("idempotent counts objects=%d references=%d", objectCount, referenceCount)
	}
}
