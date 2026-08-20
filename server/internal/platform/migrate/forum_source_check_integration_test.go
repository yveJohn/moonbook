//go:build integration

package migrate

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestForumSourceConnectionCheckMigrationScenarios(t *testing.T) {
	adminDSN := strings.TrimSpace(os.Getenv("MOONBOOK_MIGRATION_TEST_ADMIN_DSN"))
	if adminDSN == "" {
		t.Skip("MOONBOOK_MIGRATION_TEST_ADMIN_DSN 未配置")
	}
	adminDB, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer adminDB.Close()

	for _, scenario := range []struct {
		name      string
		from      int64
		wantCount int
	}{
		{name: "empty database", from: 0, wantCount: 70},
		{name: "upgrade from cookie secret reference", from: 69, wantCount: 1},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db, ctx := createMigrationTestDatabase(t, adminDB, adminDSN)
			provider, err := NewProvider(db)
			if err != nil {
				t.Fatal(err)
			}
			if scenario.from > 0 {
				if results, err := provider.UpTo(ctx, scenario.from); err != nil || len(results) != int(scenario.from) {
					t.Fatalf("migrate to %d: applied=%d err=%v", scenario.from, len(results), err)
				}
			}
			results, err := provider.UpTo(ctx, 70)
			if err != nil || len(results) != scenario.wantCount {
				t.Fatalf("migrate to 70: applied=%d err=%v", len(results), err)
			}
			verifyForumSourceConnectionCheckMigration(t, db)
			if replayed, err := provider.UpTo(ctx, 70); err != nil || len(replayed) != 0 {
				t.Fatalf("repeat migration: applied=%d err=%v", len(replayed), err)
			}
		})
	}
}

func verifyForumSourceConnectionCheckMigration(t *testing.T, db *sql.DB) {
	t.Helper()
	var description, group, method string
	if err := db.QueryRow(`SELECT description,api_group,method FROM sys_apis WHERE id=1822 AND path='/novel/crawl/sources/:id/check'`).Scan(&description, &group, &method); err != nil {
		t.Fatal(err)
	}
	if description != "检查论坛来源连接" || group != "内容采集" || method != "POST" {
		t.Fatalf("unexpected API metadata description=%q group=%q method=%q", description, group, method)
	}
	var policies int
	if err := db.QueryRow(`SELECT count(*) FROM casbin_rule WHERE ptype='p' AND v0='888' AND v1='/novel/crawl/sources/:id/check' AND v2='POST'`).Scan(&policies); err != nil || policies != 1 {
		t.Fatalf("policy count=%d err=%v", policies, err)
	}
	var sequence int64
	if err := db.QueryRow(`SELECT last_value FROM sys_apis_id_seq`).Scan(&sequence); err != nil || sequence < 1822 {
		t.Fatalf("API sequence=%d err=%v", sequence, err)
	}
}
