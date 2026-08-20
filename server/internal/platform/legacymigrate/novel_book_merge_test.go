package legacymigrate

import "testing"

func TestMapLegacyMergeTaskStatus(t *testing.T) {
	cases := map[string]struct {
		status      string
		interrupted bool
		ok          bool
	}{
		"running": {"failed", true, true},
		"pending": {"failed", true, true},
		"success": {"succeeded", false, true},
		"failed":  {"failed", false, true},
		"unknown": {"", false, false},
	}
	for input, want := range cases {
		status, interrupted, ok := mapLegacyMergeTaskStatus(input)
		if status != want.status || interrupted != want.interrupted || ok != want.ok {
			t.Fatalf("mapLegacyMergeTaskStatus(%q) = (%q,%v,%v), want (%q,%v,%v)", input, status, interrupted, ok, want.status, want.interrupted, want.ok)
		}
	}
}
