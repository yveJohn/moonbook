package legacymigrate

import "testing"

func TestMapLegacyFetchStageAndStatus(t *testing.T) {
	for _, value := range []string{"discover", "thread", "chapter", "import", "quality"} {
		if got, ok := mapLegacyFetchStage(value, false); !ok || got != value {
			t.Fatalf("stage %q = %q,%v", value, got, ok)
		}
	}
	for _, value := range []string{"fetch", "rate_limit"} {
		if got, ok := mapLegacyFetchStage(value, false); !ok || got != "discover" {
			t.Fatalf("stage %q without task = %q,%v", value, got, ok)
		}
		if got, ok := mapLegacyFetchStage(value, true); !ok || got != "thread" {
			t.Fatalf("stage %q with task = %q,%v", value, got, ok)
		}
	}
	if got, ok := mapLegacyFetchStage("parse", true); !ok || got != "chapter" {
		t.Fatalf("stage parse = %q,%v", got, ok)
	}
	for input, want := range map[string]string{"started": "started", "success": "succeeded", "succeeded": "succeeded", "failed": "failed", "skipped": "skipped"} {
		if got, ok := mapLegacyFetchStatus(input); !ok || got != want {
			t.Fatalf("status %q = %q,%v", input, got, ok)
		}
	}
	if _, ok := mapLegacyFetchStage("unknown", false); ok {
		t.Fatal("unknown stage should be rejected")
	}
	if _, ok := mapLegacyFetchStatus("unknown"); ok {
		t.Fatal("unknown status should be rejected")
	}
}
