package migrate

import (
	"database/sql"
	"io/fs"
	"regexp"
	"sort"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestEmbeddedMigrationsLoad(t *testing.T) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no embedded migrations")
	}
	db, err := sql.Open("pgx", "postgres://unused")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	provider, err := NewProvider(db)
	if err != nil {
		t.Fatalf("load embedded migrations: %v", err)
	}
	if provider == nil {
		t.Fatal("nil migration provider")
	}
}

func TestEmbeddedMigrationManifest(t *testing.T) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	want := []string{
		"00001_platform_foundation.sql",
		"00002_gva_foundation.sql",
		"00003_gva_seed.sql",
		"00004_novel_metadata.sql",
		"00005_novel_objects.sql",
		"00006_novel_object_reference_integrity.sql",
		"00007_novel_books.sql",
		"00008_novel_book_cover_api.sql",
		"00009_novel_object_source_fingerprint.sql",
		"00010_novel_chapters.sql",
		"00011_novel_reader_seo.sql",
		"00012_reader_commerce_foundation.sql",
		"00013_reader_wallet_foundation.sql",
		"00014_reader_recharge_foundation.sql",
		"00015_reader_checkin_foundation.sql",
		"00016_reader_purchase_orders.sql",
		"00017_reader_recharge_admin.sql",
		"00018_reader_payment_admin.sql",
		"00019_reader_recharge_order_admin.sql",
		"00020_reader_payment_callback_admin.sql",
		"00021_reader_wallet_admin.sql",
	}
	if strings.Join(names, "\n") != strings.Join(want, "\n") {
		t.Fatalf("migration manifest = %v, want %v", names, want)
	}
}

func TestEmbeddedMigrationsAreForwardOnlyAndContainNoSecrets(t *testing.T) {
	forbidden := regexp.MustCompile(`(?i)\b(drop\s+database|truncate|delete\s+from)\b`)
	credentialMarkers := regexp.MustCompile(`(?i)(MoonbookBaselineOnly|moonbook_local_|password\s*=)`)
	err := fs.WalkDir(migrationFS, "migrations", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		data, err := migrationFS.ReadFile(path)
		if err != nil {
			return err
		}
		sqlText := string(data)
		if forbidden.MatchString(sqlText) {
			t.Errorf("%s contains destructive SQL", path)
		}
		if credentialMarkers.MatchString(sqlText) {
			t.Errorf("%s contains a credential marker", path)
		}
		if !strings.Contains(sqlText, "Moonbook migrations are forward-only") {
			t.Errorf("%s does not declare a forward-only down migration", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGVASeedContainsOnlyFrameworkData(t *testing.T) {
	data, err := migrationFS.ReadFile("migrations/00003_gva_seed.sql")
	if err != nil {
		t.Fatal(err)
	}
	seed := string(data)
	if got := strings.Count(seed, "INSERT INTO "); got != 771 {
		t.Fatalf("seed INSERT count = %d, want 771", got)
	}
	if got := strings.Count(seed, "ON CONFLICT DO NOTHING;"); got != 771 {
		t.Fatalf("complete seed INSERT count = %d, want 771", got)
	}
	for _, table := range []string{
		"sys_users", "sys_user_authority", "sys_user_departments",
		"sys_user_positions", "media_file_upload_and_downloads",
		"media_upload_chunks", "media_uploads",
	} {
		if strings.Contains(seed, "INSERT INTO public."+table+" ") {
			t.Errorf("seed contains excluded table %s", table)
		}
	}
	if strings.Contains(seed, "/init/initdb") {
		t.Fatal("seed exposes disabled HTTP database initialization endpoint")
	}
	templateStart := strings.Index(seed, "INSERT INTO public.sys_export_templates")
	if templateStart < 0 || !strings.Contains(seed[templateStart:], "ON CONFLICT DO NOTHING;") {
		t.Fatal("multiline export template INSERT is incomplete")
	}
}
