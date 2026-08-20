package legacymigrate

import "testing"

func TestMapLegacyImportStatus(t *testing.T) {
	tests := map[string]struct {
		status string
		stop   bool
	}{"pending": {"pending", false}, "running": {"failed", true}, "success": {"succeeded", false}, "failed": {"failed", false}, "stopped": {"cancelled", false}}
	for input, want := range tests {
		status, interrupted, ok := mapLegacyImportStatus(input)
		if !ok || status != want.status || interrupted != want.stop {
			t.Fatalf("status %q = %q,%v,%v, want %q,%v,true", input, status, interrupted, ok, want.status, want.stop)
		}
	}
	if _, _, ok := mapLegacyImportStatus("unknown"); ok {
		t.Fatal("unknown import status should be rejected")
	}
}

func TestMapLegacyQualityStatus(t *testing.T) {
	for _, value := range []string{"pending", "passed", "warning", "failed"} {
		if got, ok := mapLegacyQualityStatus(value); !ok || got != value {
			t.Fatalf("quality %q = %q,%v", value, got, ok)
		}
	}
	if _, ok := mapLegacyQualityStatus("success"); ok {
		t.Fatal("unsupported quality status should be rejected")
	}
}
