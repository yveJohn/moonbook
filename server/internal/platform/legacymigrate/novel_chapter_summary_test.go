package legacymigrate

import "testing"

func TestMapLegacySummaryTaskStatus(t *testing.T) {
	cases := map[string]struct {
		status      string
		interrupted bool
		ok          bool
	}{
		"running":   {"failed", true, true},
		"completed": {"completed", false, true},
		"stopped":   {"stopped", false, true},
		"failed":    {"failed", false, true},
		"unknown":   {"", false, false},
	}
	for input, want := range cases {
		status, interrupted, ok := mapLegacySummaryTaskStatus(input)
		if status != want.status || interrupted != want.interrupted || ok != want.ok {
			t.Fatalf("mapLegacySummaryTaskStatus(%q) = (%q,%v,%v), want (%q,%v,%v)", input, status, interrupted, ok, want.status, want.interrupted, want.ok)
		}
	}
}
