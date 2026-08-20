package legacymigrate

import "testing"

func TestMapLegacyFetchStageAndStatus(t *testing.T) {
	for _, value := range []string{"discover", "thread", "chapter", "import", "quality"} {
		if got, ok := mapLegacyFetchStage(value); !ok || got != value {
			t.Fatalf("stage %q = %q,%v", value, got, ok)
		}
	}
	for input, want := range map[string]string{"started": "started", "success": "succeeded", "succeeded": "succeeded", "failed": "failed", "skipped": "skipped"} {
		if got, ok := mapLegacyFetchStatus(input); !ok || got != want {
			t.Fatalf("status %q = %q,%v", input, got, ok)
		}
	}
	if _, ok := mapLegacyFetchStage("unknown"); ok {
		t.Fatal("unknown stage should be rejected")
	}
	if _, ok := mapLegacyFetchStatus("unknown"); ok {
		t.Fatal("unknown status should be rejected")
	}
}
