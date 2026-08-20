package legacymigrate

import "testing"

func TestMapLegacyCleanTaskStatus(t *testing.T) {
	cases := []struct {
		input       string
		status      string
		interrupted bool
		ok          bool
	}{
		{"running", "failed", true, true},
		{"completed", "completed", false, true},
		{"stopped", "stopped", false, true},
		{"failed", "failed", false, true},
		{"unknown", "", false, false},
	}
	for _, tc := range cases {
		status, interrupted, ok := mapLegacyCleanTaskStatus(tc.input)
		if status != tc.status || interrupted != tc.interrupted || ok != tc.ok {
			t.Fatalf("mapLegacyCleanTaskStatus(%q) = (%q,%v,%v), want (%q,%v,%v)", tc.input, status, interrupted, ok, tc.status, tc.interrupted, tc.ok)
		}
	}
}

func TestMapLegacyCleanResultStatus(t *testing.T) {
	for input, want := range map[string]string{
		"success": "success", "discarded": "discarded", "manual_discarded": "discarded",
		"failed": "failed", "skipped": "skipped", "expired": "expired", "pending": "pending_review",
	} {
		got, ok := mapLegacyCleanResultStatus(input)
		if !ok || got != want {
			t.Fatalf("mapLegacyCleanResultStatus(%q) = (%q,%v), want (%q,true)", input, got, ok, want)
		}
	}
	if _, ok := mapLegacyCleanResultStatus("running"); ok {
		t.Fatal("running must not be accepted as a result status")
	}
}
