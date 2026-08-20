package main

import (
	"strings"
	"testing"
)

func TestConfigValidateRequiresExplicitLocalTarget(t *testing.T) {
	valid := config{
		dsn:      "postgres://moonbook:secret@127.0.0.1:25488/moonbook?sslmode=disable",
		endpoint: "127.0.0.1:29488", accessKey: "moonbook", secretKey: "secret",
		bucket: "moonbook-content", confirmation: fixtureConfirmation,
	}
	if err := valid.validate(); err != nil {
		t.Fatalf("valid local target rejected: %v", err)
	}
	checks := []struct {
		name string
		edit func(*config)
	}{
		{"confirmation", func(c *config) { c.confirmation = "" }},
		{"database host", func(c *config) { c.dsn = "postgres://moonbook:secret@db.example.com/moonbook" }},
		{"MinIO host", func(c *config) { c.endpoint = "minio.example.com:9000" }},
		{"missing secret", func(c *config) { c.secretKey = "" }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			candidate := valid
			check.edit(&candidate)
			if err := candidate.validate(); err == nil {
				t.Fatal("unsafe fixture target accepted")
			}
		})
	}
}

func TestReaderCleanupCoversAccountReferencesBeforeDeletingAccount(t *testing.T) {
	statements := readerCleanupStatements(fixtureReaderPrefix + "%")
	requiredTables := []string{
		"reader_payment_callback_logs", "reader_recharge_orders", "reader_purchase_orders",
		"commerce_entitlements", "commerce_membership_grants", "reader_checkin_records",
		"reader_invite_reward_records", "reader_wallet_adjustments", "reader_bonus_coin_buckets",
		"reader_daily_activity", "reader_wallet_ledgers", "reader_wallets", "reader_feedback",
		"reader_reading_preferences", "reader_reading_history", "reader_book_likes",
		"reader_bookshelf_entries", "reader_sessions", "reader_invite_relations",
		"reader_invite_codes", "commerce_reader_search_projection", "reader_accounts",
	}
	joined := make([]string, 0, len(statements))
	accountDelete := -1
	for index, statement := range statements {
		joined = append(joined, statement.query)
		if strings.HasPrefix(statement.query, "DELETE FROM reader_accounts ") {
			accountDelete = index
		}
	}
	queries := strings.Join(joined, "\n")
	for _, table := range requiredTables {
		if !strings.Contains(queries, "DELETE FROM "+table) {
			t.Errorf("cleanup does not cover %s", table)
		}
	}
	if accountDelete < 0 || accountDelete != len(statements)-2 {
		t.Fatalf("reader account must be deleted after dependent rows, index=%d len=%d", accountDelete, len(statements))
	}
}

func TestCallbackCleanupUsesDedicatedRequestPrefix(t *testing.T) {
	statement := callbackCleanupStatement()
	if !strings.Contains(statement.query, "request_id LIKE $1") || len(statement.args) != 1 || statement.args[0] != fixtureCallbackPrefix+"%" {
		t.Fatalf("callback cleanup=%+v", statement)
	}
}
