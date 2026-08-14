package legacymigrate

import "testing"

func TestReaderMigrationMappings(t *testing.T) {
	for _, value := range []string{"days_7", "days_30", "days_90", "days_365", "permanent"} {
		if !validLegacyMembershipGrantType(value) {
			t.Fatalf("expected membership grant type %q to be valid", value)
		}
	}
	if validLegacyMembershipGrantType("unknown") {
		t.Fatal("unsupported membership grant type was accepted")
	}

	for source, target := range map[string]string{"confirmed": "active", "no_change": "disabled"} {
		got, ok := mapLegacyMembershipStatus(source)
		if !ok || got != target {
			t.Fatalf("membership status %q = %q, %v", source, got, ok)
		}
	}
	if _, ok := mapLegacyMembershipStatus("failed"); ok {
		t.Fatal("unsupported membership status was accepted")
	}

	validEntitlements := []struct {
		kind   string
		target int64
	}{{"book", 1}, {"chapter", 1}, {"ad_free", 0}, {"membership", 0}}
	for _, value := range validEntitlements {
		if !validEntitlementType(value.kind, value.target) {
			t.Fatalf("expected entitlement %q/%d to be valid", value.kind, value.target)
		}
	}
	if validEntitlementType("book", 0) || validEntitlementType("membership", 1) || validEntitlementType("unknown", 0) {
		t.Fatal("invalid entitlement type or target was accepted")
	}
}
