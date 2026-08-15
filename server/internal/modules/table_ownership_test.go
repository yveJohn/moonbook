package modules_test

import "testing"

func TestTableOwnershipRegistryCoversDomainPrefixes(t *testing.T) {
	for table, owner := range tableOwners {
		if owner != readerOwner && owner != commerceOwner && owner != novelOwner {
			t.Errorf("%s: invalid owner %q", table, owner)
		}
	}
	for _, table := range []string{
		"reader_accounts",
		"reader_invite_relations",
		"reader_wallets",
		"reader_purchase_orders",
		"commerce_reader_search_projection",
		"novel_books",
		"novel_chapters",
		"novel_objects",
	} {
		if tableOwners[table] == "" {
			t.Errorf("ownership registry is missing critical table %s", table)
		}
	}
}
