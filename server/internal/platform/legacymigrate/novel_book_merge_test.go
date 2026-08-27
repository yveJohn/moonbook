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

func TestMapLegacyMergeSortTimeSource(t *testing.T) {
	tests := map[string]string{
		"thread_create_time":        "thread_created_at",
		"thread_created_at":         "thread_created_at",
		"import_task_create_time":   "import_task_created_at",
		"import_task_created_at":    "import_task_created_at",
		" IMPORT_TASK_CREATE_TIME ": "import_task_created_at",
	}
	for input, want := range tests {
		got, ok := mapLegacyMergeSortTimeSource(input)
		if !ok || got != want {
			t.Fatalf("mapLegacyMergeSortTimeSource(%q) = (%q,%v), want (%q,true)", input, got, ok, want)
		}
	}
	if got, ok := mapLegacyMergeSortTimeSource("unknown"); ok || got != "" {
		t.Fatalf("mapLegacyMergeSortTimeSource(unknown) = (%q,%v), want (\"\",false)", got, ok)
	}
}
