package legacymigrate

import "testing"

func TestMapLegacyCandidateStatus(t *testing.T) {
	tests := map[string]string{
		"pending": "pending", "running": "importing", "success": "imported",
		"succeeded": "imported", "stopped": "failed", "cancelled": "failed",
	}
	for input, want := range tests {
		got, ok := mapLegacyCandidateStatus(input)
		if !ok || got != want {
			t.Fatalf("status %q = %q,%v, want %q,true", input, got, ok, want)
		}
	}
	if _, ok := mapLegacyCandidateStatus("unknown"); ok {
		t.Fatal("unknown status should be rejected")
	}
}

func TestValidateLegacyCandidate(t *testing.T) {
	item := legacyCrawlCandidate{id: 1, sourceID: 2, boardID: 3, threadID: "thread", title: "title", threadURL: "https://example.test/thread", followCount: 0}
	if code, message := validateLegacyCandidate(item); code != "" || message != "" {
		t.Fatalf("valid candidate rejected: %s %s", code, message)
	}
	item.threadURL = ""
	if code, _ := validateLegacyCandidate(item); code != "INVALID_CANDIDATE_TEXT" {
		t.Fatalf("invalid URL code=%s", code)
	}
}
