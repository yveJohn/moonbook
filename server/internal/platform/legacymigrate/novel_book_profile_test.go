package legacymigrate

import "testing"

func TestMapLegacyProfileStatus(t *testing.T) {
	for input, want := range map[string]string{
		"running": "running", "pending": "pending", "approved": "approved",
		"rejected": "rejected", "applied": "applied", "failed": "failed",
		"retried": "retried", "recovered": "recovered", "expired": "expired",
	} {
		got, ok := mapLegacyProfileStatus(input)
		if !ok || got != want {
			t.Fatalf("mapLegacyProfileStatus(%q) = (%q,%v), want (%q,true)", input, got, ok, want)
		}
	}
	if _, ok := mapLegacyProfileStatus("unknown"); ok {
		t.Fatal("unknown profile status must be rejected")
	}
}

func TestJSONValueNeverReturnsRawInvalidJSON(t *testing.T) {
	if got := jsonValue(`{"category":"fiction"}`); got == nil {
		t.Fatal("valid JSON should be retained")
	}
	if got := jsonValue("not-json"); got == nil {
		t.Fatal("invalid JSON should be converted to an empty array")
	}
}
