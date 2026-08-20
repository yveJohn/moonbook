//go:build integration

package legacyaudit_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/legacyaudit"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/legacymigrate"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/migrate"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestContentAuditDetectsBrokenRelationAndCheckFailure(t *testing.T) {
	adminDSN := strings.TrimSpace(os.Getenv("MOONBOOK_MIGRATION_TEST_ADMIN_DSN"))
	if adminDSN == "" {
		t.Skip("MOONBOOK_MIGRATION_TEST_ADMIN_DSN is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	databaseName := fmt.Sprintf("moonbook_content_audit_it_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, `CREATE DATABASE "`+databaseName+`"`); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("pgx", integrationDatabaseDSN(t, adminDSN, databaseName))
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
		t.Fatalf("migrate content audit database: applied=%d err=%v", len(results), err)
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE novel_crawl_forum_board DROP CONSTRAINT novel_crawl_forum_board_source_id_fkey`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_crawl_forum_board(source_id,source_name,board_name,board_url) VALUES(999999,'missing','broken','https://forum.example.test/broken')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `DROP TABLE novel_ai_config_model CASCADE`); err != nil {
		t.Fatal(err)
	}
	issues := legacyaudit.AuditTargetIntegrity(ctx, db)
	joined := strings.Join(issues, "\n")
	if !strings.Contains(joined, "crawl_board_missing_source=1") {
		t.Fatalf("issues=%v", issues)
	}
	if !strings.Contains(joined, "check_query_failed:ai_model_missing_config") {
		t.Fatalf("issues=%v", issues)
	}
}

func TestDualDatabaseAuditDetectsSourceTargetRowAndPrimaryKeyMismatch(t *testing.T) {
	legacyDSN := strings.TrimSpace(os.Getenv("MOONBOOK_LEGACY_TEST_ADMIN_DSN"))
	adminDSN := strings.TrimSpace(os.Getenv("MOONBOOK_MIGRATION_TEST_ADMIN_DSN"))
	if legacyDSN == "" || adminDSN == "" {
		t.Skip("legacy MySQL and PostgreSQL admin DSNs are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	source, err := sql.Open("mysql", legacyDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if _, err := source.ExecContext(ctx, `CREATE TABLE novel_crawl_forum_source(id bigint PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := source.ExecContext(ctx, `INSERT INTO novel_crawl_forum_source(id) VALUES(9007199254740993)`); err != nil {
		t.Fatal(err)
	}
	if _, err := source.ExecContext(ctx, `SET GLOBAL read_only=ON`); err != nil {
		t.Fatal(err)
	}
	if err := legacymigrate.VerifySourceReadOnly(ctx, source); err != nil {
		t.Fatal(err)
	}
	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	databaseName := fmt.Sprintf("moonbook_dual_audit_it_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, `CREATE DATABASE "`+databaseName+`"`); err != nil {
		t.Fatal(err)
	}
	target, err := sql.Open("pgx", integrationDatabaseDSN(t, adminDSN, databaseName))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = target.Close()
		_, _ = admin.ExecContext(context.Background(), `DROP DATABASE IF EXISTS "`+databaseName+`" WITH (FORCE)`)
	}()
	provider, err := migrate.NewProvider(target)
	if err != nil {
		t.Fatal(err)
	}
	if results, err := provider.Up(ctx); err != nil || len(results) != 70 {
		t.Fatalf("migrate dual audit database: applied=%d err=%v", len(results), err)
	}
	report, err := legacyaudit.Build(ctx, source, target, "moonbook-v1")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, mapping := range report.Mappings {
		if mapping.Mapping.Source != "novel_crawl_forum_source" {
			continue
		}
		found = true
		joined := strings.Join(mapping.Errors, ",")
		if mapping.SourceRows != 1 || mapping.TargetRows != 0 || !strings.Contains(joined, "target_row_count_below_source") || !strings.Contains(joined, "target_max_id_below_source") {
			t.Fatalf("mapping=%+v", mapping)
		}
	}
	if !found || !report.HasFailures() {
		t.Fatalf("audit silently passed report=%+v", report)
	}
}
