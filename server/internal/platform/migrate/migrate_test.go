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
		"00022_reader_user_admin.sql",
		"00023_reader_feedback_admin.sql",
		"00024_reader_invite_admin.sql",
		"00025_reader_checkin_admin.sql",
		"00026_reader_recharge_settings_admin.sql",
		"00027_reader_user_password_admin.sql",
		"00028_reader_wallet_adjust_admin.sql",
		"00029_reader_recharge_order_manual_admin.sql",
		"00030_reader_payment_channel_check.sql",
		"00031_reader_product_admin.sql",
		"00032_reader_purchase_order_admin.sql",
		"00033_reader_membership_grant_admin.sql",
		"00034_reader_invite_reward_admin.sql",
		"00035_reader_recharge_order_sync.sql",
		"00036_novel_crawl_forum_source.sql",
		"00037_novel_crawl_forum_board_admin.sql",
		"00038_novel_crawl_thread_candidate.sql",
		"00039_novel_crawl_import_task.sql",
		"00040_novel_crawl_fetch_log.sql",
		"00041_novel_crawl_candidate_discovery.sql",
		"00042_novel_txt_import_task.sql",
		"00043_novel_txt_import_preview.sql",
		"00044_novel_txt_import_repair.sql",
		"00045_novel_txt_import_menu_repair.sql",
		"00046_novel_book_merge.sql",
		"00047_novel_book_merge_target_fks.sql",
		"00048_novel_ai_config.sql",
		"00049_novel_chapter_clean.sql",
		"00050_novel_chapter_summary.sql",
		"00051_novel_chapter_clean_object_kind.sql",
		"00052_novel_chapter_summary_task_detail.sql",
		"00053_novel_book_profile.sql",
		"00054_reader_finance_migration_support.sql",
		"00055_reader_daily_activity_dashboard.sql",
		"00056_reader_mock_recharge.sql",
		"00057_reader_invite_edit.sql",
		"00058_casbin_policy_unique.sql",
		"00059_commerce_reader_search_projection.sql",
		"00060_epusdt_payment_creation.sql",
		"00061_epusdt_callback_attempt_audit.sql",
		"00062_commerce_operations_audit.sql",
	}
	if strings.Join(names, "\n") != strings.Join(want, "\n") {
		t.Fatalf("migration manifest = %v, want %v", names, want)
	}
}

func TestEPUSDTCallbackAttemptAuditMigrationContract(t *testing.T) {
	data, err := migrationFS.ReadFile("migrations/00061_epusdt_callback_attempt_audit.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration := strings.ToLower(string(data))

	for _, required := range []string{
		"add column failure_code varchar(64)",
		"add column request_id varchar(64)",
		"add column trace_id varchar(64)",
		"add column payload_bytes integer",
		"add column payload_truncated boolean",
		"add column completed_at timestamptz",
		"reader_payment_callback_logs_payload_bytes_check",
		"payload_bytes between 0 and 16385",
		"reader_payment_callback_logs_failure_code_check",
		"create index reader_payment_callback_logs_result_created_idx",
		"create index reader_payment_callback_logs_failure_created_idx",
		"create index reader_payment_callback_logs_response_created_idx",
		"create index reader_payment_callback_logs_request_time_idx",
		"create index reader_payment_callback_logs_runtime_received_idx",
		"where source_type = 'runtime' and processing_result = 'received'",
		"moonbook migrations are forward-only",
	} {
		if !strings.Contains(migration, required) {
			t.Errorf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"update reader_payment_callback_logs", "delete from reader_payment_callback_logs"} {
		if strings.Contains(migration, forbidden) {
			t.Errorf("migration rewrites historical callback logs with %q", forbidden)
		}
	}
}

func TestEPUSDTPaymentCreationMigrationContract(t *testing.T) {
	data, err := migrationFS.ReadFile("migrations/00060_epusdt_payment_creation.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration := strings.ToLower(string(data))

	for _, required := range []string{
		"add column credential_ref varchar(100)",
		"add column merchant_pid_snapshot varchar(255)",
		"add column active_reader_id bigint references reader_accounts(id)",
		"row_number() over",
		"partition by reader_id",
		"order by created_at desc, id desc",
		"status = 'superseded'",
		"failure_code = 'order_replaced'",
		"status in ('creating', 'pending', 'gateway_unknown')",
		"active_reader_id = reader_id",
		"create unique index reader_recharge_orders_gateway_trade_id_uidx",
		"where gateway_trade_id is not null and btrim(gateway_trade_id) <> ''",
		"create unique index reader_recharge_orders_block_transaction_id_uidx",
		"where block_transaction_id is not null and btrim(block_transaction_id) <> ''",
		"create unique index reader_recharge_orders_active_reader_id_uidx",
		"where active_reader_id is not null",
		"moonbook migrations are forward-only",
	} {
		if !strings.Contains(migration, required) {
			t.Errorf("migration missing %q", required)
		}
	}
}

func TestCommerceReaderSearchProjectionMigrationBoundary(t *testing.T) {
	data, err := migrationFS.ReadFile("migrations/00059_commerce_reader_search_projection.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration := string(data)

	createTable := regexp.MustCompile(`(?is)CREATE\s+TABLE\s+commerce_reader_search_projection\s*\((.*?)\);`).FindStringSubmatch(migration)
	if len(createTable) != 2 {
		t.Fatal("commerce reader search projection table definition not found")
	}
	columnPattern := regexp.MustCompile(`(?m)^\s{4}([a-z][a-z0-9_]*)\s+(?:bigint|varchar\((?:16|64)\)|timestamptz)(?:\s|$)`)
	matches := columnPattern.FindAllStringSubmatch(createTable[1], -1)
	columns := make([]string, 0, len(matches))
	for _, match := range matches {
		columns = append(columns, match[1])
	}
	wantColumns := []string{"reader_id", "username", "nickname", "status", "created_at", "updated_at"}
	if strings.Join(columns, ",") != strings.Join(wantColumns, ",") {
		t.Fatalf("projection columns = %v, want %v", columns, wantColumns)
	}

	lower := strings.ToLower(migration)
	for _, required := range []string{
		"reader_id bigint primary key",
		"commerce_reader_search_projection_username_not_blank",
		"commerce_reader_search_projection_status_check",
		"commerce_reader_search_projection_username_idx",
		"commerce_reader_search_projection_nickname_idx",
		"commerce_reader_search_projection_status_idx",
		"insert into commerce_reader_search_projection",
		"from reader_accounts",
		"on conflict (reader_id) do update",
		"moonbook migrations are forward-only",
	} {
		if !strings.Contains(lower, required) {
			t.Errorf("migration missing %q", required)
		}
	}
	if strings.Contains(lower, "references reader_accounts") {
		t.Fatal("projection must not create a cross-domain foreign key")
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
