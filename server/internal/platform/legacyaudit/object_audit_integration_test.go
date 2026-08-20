//go:build integration

package legacyaudit_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/legacyaudit"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/migrate"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestObjectAuditDetectsRealPostgresAndMinIOFaults(t *testing.T) {
	adminDSN := strings.TrimSpace(os.Getenv("MOONBOOK_MIGRATION_TEST_ADMIN_DSN"))
	endpoint := strings.TrimSpace(os.Getenv("MOONBOOK_MINIO_ENDPOINT"))
	accessKey := strings.TrimSpace(os.Getenv("MINIO_ROOT_USER"))
	secretKey := strings.TrimSpace(os.Getenv("MINIO_ROOT_PASSWORD"))
	bucket := strings.TrimSpace(os.Getenv("MINIO_BUCKET"))
	if adminDSN == "" || endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		t.Skip("PostgreSQL admin DSN and MinIO integration settings are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	databaseName := fmt.Sprintf("moonbook_object_audit_it_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, `CREATE DATABASE "`+databaseName+`"`); err != nil {
		t.Fatal(err)
	}
	databaseDSN := integrationDatabaseDSN(t, adminDSN, databaseName)
	db, err := sql.Open("pgx", databaseDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = db.Close()
		_, _ = admin.ExecContext(context.Background(), `DROP DATABASE IF EXISTS "`+databaseName+`" WITH (FORCE)`)
	}()
	provider, err := migrate.NewProvider(db)
	if err != nil {
		t.Fatal(err)
	}
	if results, err := provider.Up(ctx); err != nil || len(results) != 70 {
		t.Fatalf("migrate object audit database: applied=%d err=%v", len(results), err)
	}
	store, err := objectstore.NewMinIOStore(objectstore.MinIOConfig{Endpoint: endpoint, AccessKey: accessKey, SecretKey: secretKey, Bucket: bucket})
	if err != nil {
		t.Fatal(err)
	}
	prefix := "audit-fixtures/" + databaseName + "/"
	keys := []string{prefix + "valid.txt", prefix + "wrong-size.txt", prefix + "wrong-hash.txt"}
	for _, key := range keys {
		key := key
		t.Cleanup(func() { _ = store.Remove(context.Background(), key) })
	}
	valid := []byte("valid object")
	validHash := sha256Hex(valid)
	for index, fixture := range []struct {
		key  string
		data []byte
		hash string
		size int64
	}{
		{keys[0], valid, validHash, int64(len(valid))},
		{keys[1], []byte("larger"), sha256Hex([]byte("larger")), 3},
		{keys[2], []byte("actual"), strings.Repeat("a", 64), int64(len("actual"))},
	} {
		actualHash := sha256Hex(fixture.data)
		if err := store.Put(ctx, fixture.key, bytes.NewReader(fixture.data), int64(len(fixture.data)), "text/plain", actualHash); err != nil {
			t.Fatal(err)
		}
		objectID := int64(index + 1)
		if _, err := db.ExecContext(ctx, `INSERT INTO novel_objects(id,object_kind,book_id,owner_id,version,object_key,sha256,byte_size,content_type,state,source,source_fingerprint) VALUES($1,'chapter_content',1,$2,1,$3,$4,$5,'text/plain','active','legacy',$6)`, objectID, objectID, fixture.key, fixture.hash, fixture.size, sha256Hex([]byte(fixture.key))); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO novel_object_references(object_kind,book_id,owner_id,object_id) VALUES('chapter_content',1,$1,$1)`, objectID); err != nil {
			t.Fatal(err)
		}
	}
	missingKey := prefix + "missing.txt"
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_objects(id,object_kind,book_id,owner_id,version,object_key,sha256,byte_size,content_type,state,source,source_fingerprint) VALUES(4,'chapter_content',1,4,1,$1,$2,1,'text/plain','active','legacy',$3)`, missingKey, strings.Repeat("b", 64), sha256Hex([]byte(missingKey))); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_object_references(object_kind,book_id,owner_id,object_id) VALUES('chapter_content',1,4,4)`); err != nil {
		t.Fatal(err)
	}

	report, err := legacyaudit.AuditObjects(ctx, db, store)
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"OBJECT_MISSING", "STAT_SIZE_MISMATCH", "STAT_HASH_MISMATCH", "CONTENT_SIZE_MISMATCH", "CONTENT_HASH_MISMATCH"} {
		if report.Issues[code] == 0 {
			t.Fatalf("report issues=%v missing=%s", report.Issues, code)
		}
	}
	if report.Checked != 4 || report.IssueCount < 5 {
		t.Fatalf("report=%+v", report)
	}
}

func integrationDatabaseDSN(t *testing.T, adminDSN, database string) string {
	t.Helper()
	parsed, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/" + database
	return parsed.String()
}

func sha256Hex(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
