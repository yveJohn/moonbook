package migrations

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestReaderCommerceMigration(t *testing.T) {
	data, err := os.ReadFile("00012_reader_commerce_foundation.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(data)
	if !strings.Contains(sql, "-- +goose Up") || !strings.Contains(sql, "-- +goose Down") {
		t.Fatal("migration must use goose up/down sections")
	}
	if !regexp.MustCompile(`(?i)\b(drop\s+database|truncate|delete\s+from)\b`).MatchString(sql) {
		// expected: no destructive statements
	} else {
		t.Fatal("migration contains destructive SQL")
	}
	for _, table := range []string{
		"reader_accounts", "reader_sessions", "reader_invite_codes", "reader_invite_relations",
		"reader_bookshelf_entries", "reader_book_likes", "reader_reading_history",
		"reader_reading_preferences", "reader_feedback", "commerce_products",
		"commerce_chapter_pricing_config", "commerce_membership_grants", "commerce_entitlements",
	} {
		if !strings.Contains(sql, "CREATE TABLE "+table) {
			t.Errorf("missing table %s", table)
		}
	}
	for _, required := range []string{"bigint", "timestamptz", "reader_invite_codes_used_check", "commerce_entitlements_status_check", "UNIQUE", "forward-only"} {
		if !strings.Contains(strings.ToLower(sql), strings.ToLower(required)) {
			t.Errorf("missing migration requirement %q", required)
		}
	}
	for _, forbidden := range []string{"reader_wallet", "reader_order", "reader_payment"} {
		if strings.Contains(strings.ToLower(sql), forbidden) {
			t.Errorf("M3 migration must not create %s", forbidden)
		}
	}
}
