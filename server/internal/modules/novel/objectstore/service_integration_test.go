package objectstore

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestVersionedObjectsWithPostgresAndMinIO(t *testing.T) {
	postgresDSN := os.Getenv("MOONBOOK_OBJECT_TEST_DSN")
	endpoint := os.Getenv("MOONBOOK_OBJECT_TEST_MINIO_ENDPOINT")
	accessKey := os.Getenv("MOONBOOK_OBJECT_TEST_MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MOONBOOK_OBJECT_TEST_MINIO_SECRET_KEY")
	if postgresDSN == "" || endpoint == "" || accessKey == "" || secretKey == "" {
		t.Skip("Moonbook object integration test environment is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := sql.Open("pgx", postgresDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	bucket := fmt.Sprintf("moonbook-object-test-%d", time.Now().UnixNano())
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, "")})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		exists, existsErr := client.BucketExists(ctx, bucket)
		if existsErr != nil || !exists {
			t.Fatalf("create test bucket: %v (exists error: %v)", err, existsErr)
		}
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		for object := range client.ListObjects(cleanupCtx, bucket, minio.ListObjectsOptions{Recursive: true}) {
			if object.Err == nil {
				_ = client.RemoveObject(cleanupCtx, bucket, object.Key, minio.RemoveObjectOptions{})
			}
		}
		_ = client.RemoveBucket(cleanupCtx, bucket)
	})
	store, err := NewMinIOStore(MinIOConfig{Endpoint: endpoint, AccessKey: accessKey, SecretKey: secretKey, Bucket: bucket})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(db, store)
	suffix := time.Now().UnixNano()
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		rows, err := db.QueryContext(cleanupCtx, `SELECT object_key FROM novel_objects WHERE book_id BETWEEN $1 AND $2`, suffix, suffix+3)
		if err != nil {
			t.Errorf("list test objects for cleanup: %v", err)
			return
		}
		var keys []string
		for rows.Next() {
			var key string
			if err := rows.Scan(&key); err != nil {
				t.Errorf("scan test object for cleanup: %v", err)
				rows.Close()
				return
			}
			keys = append(keys, key)
		}
		if err := rows.Close(); err != nil {
			t.Errorf("close test object cleanup rows: %v", err)
			return
		}
		for _, key := range keys {
			if err := store.Remove(cleanupCtx, key); err != nil {
				t.Errorf("remove test object %s: %v", key, err)
			}
		}
		for _, query := range []string{
			`DELETE FROM novel_object_references WHERE book_id BETWEEN $1 AND $2`,
			`DELETE FROM novel_object_events WHERE object_id IN (SELECT id FROM novel_objects WHERE book_id BETWEEN $1 AND $2)`,
			`DELETE FROM novel_objects WHERE book_id BETWEEN $1 AND $2`,
		} {
			if _, err := db.ExecContext(cleanupCtx, query, suffix, suffix+3); err != nil {
				t.Errorf("clean test object rows: %v", err)
			}
		}
	}()
	target := Target{Kind: KindChapterContent, BookID: suffix, OwnerID: suffix}

	first, err := service.UploadVerified(ctx, target, []byte("第一版正文\n"), "text/plain; charset=utf-8")
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != 1 || first.State != StateVerified || first.Key != fmt.Sprintf("chapters/%d/%d/v1.txt", suffix, suffix) {
		t.Fatalf("first object = %+v", first)
	}
	assertNoReference(t, ctx, db, first.ID)
	if err := service.Activate(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	data, active, err := service.ReadActive(ctx, target)
	if err != nil || string(data) != "第一版正文\n" || active.ID != first.ID {
		t.Fatalf("first active object=%+v data=%q err=%v", active, data, err)
	}

	second, err := service.UploadVerified(ctx, target, []byte("第二版正文，数据库提交前不可见\n"), "text/plain; charset=utf-8")
	if err != nil {
		t.Fatal(err)
	}
	if second.Version != 2 || second.Key == first.Key {
		t.Fatalf("second object = %+v", second)
	}
	data, active, err = service.ReadActive(ctx, target)
	if err != nil || string(data) != "第一版正文\n" || active.ID != first.ID {
		t.Fatalf("uncommitted second version became visible: active=%+v data=%q err=%v", active, data, err)
	}
	if err := service.Activate(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	data, active, err = service.ReadActive(ctx, target)
	if err != nil || !strings.HasPrefix(string(data), "第二版正文") || active.ID != second.ID {
		t.Fatalf("second active object=%+v data=%q err=%v", active, data, err)
	}
	var firstState string
	if err := db.QueryRowContext(ctx, `SELECT state FROM novel_objects WHERE id=$1`, first.ID).Scan(&firstState); err != nil || firstState != StateOrphaned {
		t.Fatalf("first state=%q err=%v", firstState, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE novel_objects SET orphaned_at=now()-interval '2 hours' WHERE id=$1`, first.ID); err != nil {
		t.Fatal(err)
	}
	result, err := service.Collect(ctx, time.Now().Add(-time.Hour), 100)
	if err != nil || result.Deleted != 1 || result.Failed != 0 {
		t.Fatalf("collect result=%+v err=%v", result, err)
	}
	if _, err := store.Stat(ctx, first.Key); err == nil {
		t.Fatal("orphaned first object still exists")
	}
	if _, err := store.Stat(ctx, second.Key); err != nil {
		t.Fatalf("active second object was collected: %v", err)
	}
	var deletedEvents int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM novel_object_events WHERE object_id=$1 AND event_type='deleted'`, first.ID).Scan(&deletedEvents); err != nil || deletedEvents != 1 {
		t.Fatalf("deleted events=%d err=%v", deletedEvents, err)
	}

	third, err := service.UploadVerified(ctx, target, []byte("并发版本三"), "text/plain; charset=utf-8")
	if err != nil {
		t.Fatal(err)
	}
	fourth, err := service.UploadVerified(ctx, target, []byte("并发版本四"), "text/plain; charset=utf-8")
	if err != nil {
		t.Fatal(err)
	}
	errorsCh := make(chan error, 2)
	go func() { errorsCh <- service.Activate(ctx, third.ID) }()
	go func() { errorsCh <- service.Activate(ctx, fourth.ID) }()
	for range 2 {
		if err := <-errorsCh; err != nil {
			t.Fatalf("concurrent activation: %v", err)
		}
	}
	var activeCount, orphanedCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FILTER (WHERE state='active'),count(*) FILTER (WHERE state='orphaned')
		FROM novel_objects WHERE id IN ($1,$2)`, third.ID, fourth.ID).Scan(&activeCount, &orphanedCount); err != nil {
		t.Fatal(err)
	}
	if activeCount != 1 || orphanedCount != 1 {
		t.Fatalf("concurrent states active=%d orphaned=%d", activeCount, orphanedCount)
	}

	corruptTarget := Target{Kind: KindBookCover, BookID: suffix + 1, OwnerID: suffix + 1, Extension: "webp"}
	corruptService := NewService(db, corruptStatStore{BlobStore: store})
	if _, err := corruptService.UploadVerified(ctx, corruptTarget, []byte("fake-webp"), "image/webp"); err == nil {
		t.Fatal("corrupt stat should reject upload verification")
	}
	var failedID int64
	var failedKey, failedState string
	if err := db.QueryRowContext(ctx, `SELECT id,object_key,state FROM novel_objects
		WHERE object_kind=$1 AND book_id=$2 AND owner_id=$3 ORDER BY id DESC LIMIT 1`,
		corruptTarget.Kind, corruptTarget.BookID, corruptTarget.OwnerID).Scan(&failedID, &failedKey, &failedState); err != nil {
		t.Fatal(err)
	}
	if failedState != StateFailed {
		t.Fatalf("corrupt object state=%q", failedState)
	}
	assertNoReference(t, ctx, db, failedID)
	if _, err := db.ExecContext(ctx, `UPDATE novel_objects SET created_at=now()-interval '2 hours' WHERE id=$1`, failedID); err != nil {
		t.Fatal(err)
	}
	result, err = service.Collect(ctx, time.Now().Add(-time.Hour), 100)
	if err != nil || result.Deleted < 1 {
		t.Fatalf("collect corrupt object result=%+v err=%v", result, err)
	}
	if _, err := store.Stat(ctx, failedKey); err == nil {
		t.Fatal("failed verification object still exists")
	}

	coverTarget := Target{Kind: KindBookCover, BookID: suffix + 2, OwnerID: suffix + 2, Extension: "png"}
	coverOne, err := service.UploadVerified(ctx, coverTarget, []byte("png-cover-v1"), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if coverOne.Key != fmt.Sprintf("covers/%d/v1.png", suffix+2) {
		t.Fatalf("first cover key=%q", coverOne.Key)
	}
	if err := service.Activate(ctx, coverOne.ID); err != nil {
		t.Fatal(err)
	}
	coverTarget.Extension = "webp"
	coverTwo, err := service.UploadVerified(ctx, coverTarget, []byte("webp-cover-v2"), "image/webp")
	if err != nil {
		t.Fatal(err)
	}
	if coverTwo.Key != fmt.Sprintf("covers/%d/v2.webp", suffix+2) {
		t.Fatalf("second cover key=%q", coverTwo.Key)
	}
	if err := service.Activate(ctx, coverTwo.ID); err != nil {
		t.Fatal(err)
	}
	data, active, err = service.ReadActive(ctx, coverTarget)
	if err != nil || string(data) != "webp-cover-v2" || active.ID != coverTwo.ID || active.ContentType != "image/webp" {
		t.Fatalf("active cover=%+v data=%q err=%v", active, data, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT state FROM novel_objects WHERE id=$1`, coverOne.ID).Scan(&firstState); err != nil || firstState != StateOrphaned {
		t.Fatalf("previous cover state=%q err=%v", firstState, err)
	}

	fingerprintTarget := Target{Kind: KindBookCover, BookID: suffix + 3, OwnerID: suffix + 3, Extension: "png"}
	fingerprint := strings.Repeat("a", 64)
	start := make(chan struct{})
	results := make(chan Object, 2)
	errors := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			object, err := service.UploadVerifiedWithOptions(ctx, fingerprintTarget, []byte("legacy-concurrent-cover"), "image/png", UploadOptions{Source: "legacy", SourceFingerprint: fingerprint})
			results <- object
			errors <- err
		}()
	}
	close(start)
	firstConcurrent, secondConcurrent := <-results, <-results
	if err := <-errors; err != nil {
		t.Fatal(err)
	}
	if err := <-errors; err != nil {
		t.Fatal(err)
	}
	if firstConcurrent.ID != secondConcurrent.ID {
		t.Fatalf("concurrent fingerprint objects differ: %d != %d", firstConcurrent.ID, secondConcurrent.ID)
	}
	var fingerprintCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM novel_objects WHERE book_id=$1 AND source_fingerprint=$2`, fingerprintTarget.BookID, fingerprint).Scan(&fingerprintCount); err != nil || fingerprintCount != 1 {
		t.Fatalf("fingerprint object count=%d err=%v", fingerprintCount, err)
	}
}

type corruptStatStore struct {
	BlobStore
}

func (store corruptStatStore) Stat(ctx context.Context, key string) (BlobStat, error) {
	stat, err := store.BlobStore.Stat(ctx, key)
	if err == nil {
		stat.SHA256 = strings.Repeat("0", 64)
	}
	return stat, err
}

func assertNoReference(t *testing.T, ctx context.Context, db *sql.DB, objectID int64) {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM novel_object_references WHERE object_id=$1`, objectID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("object %d reference count=%d err=%v", objectID, count, err)
	}
}
