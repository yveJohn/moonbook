//go:build integration

package migrate

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestForumCookieSecretReferenceMigrationScenarios(t *testing.T) {
	adminDSN := strings.TrimSpace(os.Getenv("MOONBOOK_MIGRATION_TEST_ADMIN_DSN"))
	if adminDSN == "" {
		t.Skip("MOONBOOK_MIGRATION_TEST_ADMIN_DSN 未配置")
	}
	adminDB, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer adminDB.Close()

	t.Run("empty database and replay", func(t *testing.T) {
		db, ctx := createMigrationTestDatabase(t, adminDB, adminDSN)
		provider, err := NewProvider(db)
		if err != nil {
			t.Fatal(err)
		}
		results, err := provider.UpTo(ctx, 69)
		if err != nil || len(results) != 69 {
			t.Fatalf("empty migration: applied=%d err=%v", len(results), err)
		}
		if replayed, err := provider.UpTo(ctx, 69); err != nil || len(replayed) != 0 {
			t.Fatalf("repeat migration: applied=%d err=%v", len(replayed), err)
		}
	})

	t.Run("upgrade clears plaintext and records only hashes", func(t *testing.T) {
		db, ctx := createMigrationTestDatabase(t, adminDB, adminDSN)
		provider, err := NewProvider(db)
		if err != nil {
			t.Fatal(err)
		}
		if results, err := provider.UpTo(ctx, 68); err != nil || len(results) != 68 {
			t.Fatalf("migrate to 68: applied=%d err=%v", len(results), err)
		}
		var sourceID int64
		if err := db.QueryRowContext(ctx, `INSERT INTO novel_crawl_forum_source(source_name,base_url,cookie_text) VALUES('migration-cookie-fixture','https://forum.example.test','session=migration-fixture') RETURNING id`).Scan(&sourceID); err != nil {
			t.Fatal(err)
		}
		var before int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM novel_crawl_forum_source`).Scan(&before); err != nil {
			t.Fatal(err)
		}
		if results, err := provider.UpTo(ctx, 69); err != nil || len(results) != 1 {
			t.Fatalf("migrate to 69: applied=%d err=%v", len(results), err)
		}
		var cookie sql.NullString
		var ref string
		if err := db.QueryRowContext(ctx, `SELECT cookie_text,cookie_secret_ref FROM novel_crawl_forum_source WHERE id=$1`, sourceID).Scan(&cookie, &ref); err != nil || cookie.Valid || ref != "" {
			t.Fatalf("plaintext state cookiePresent=%t ref=%q err=%v", cookie.Valid, ref, err)
		}
		var after, auditCount, hashLength int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM novel_crawl_forum_source`).Scan(&after); err != nil || after != before {
			t.Fatalf("source rows before=%d after=%d err=%v", before, after, err)
		}
		if err := db.QueryRowContext(ctx, `SELECT count(*),COALESCE(min(length(source_id_hash)),0) FROM novel_crawl_cookie_secret_migration_audit WHERE had_legacy_cookie`).Scan(&auditCount, &hashLength); err != nil || auditCount != 1 || hashLength != 64 {
			t.Fatalf("audit count=%d hashLength=%d err=%v", auditCount, hashLength, err)
		}
		if _, err := db.ExecContext(ctx, `UPDATE novel_crawl_forum_source SET cookie_text='rejected' WHERE id=$1`, sourceID); err == nil {
			t.Fatal("plaintext constraint accepted a cookie")
		}
	})
}
