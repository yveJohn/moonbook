package legacyaudit

import (
	"strings"
	"testing"
)

func TestMappingsHaveUniqueStagesAndTables(t *testing.T) {
	seen := map[string]bool{}
	for _, mapping := range Mappings() {
		if strings.TrimSpace(mapping.Source) == "" || strings.TrimSpace(mapping.Target) == "" || strings.TrimSpace(mapping.Stage) == "" {
			t.Fatalf("incomplete mapping: %+v", mapping)
		}
		key := mapping.Source + "->" + mapping.Target
		if seen[key] {
			t.Fatalf("duplicate mapping %s", key)
		}
		seen[key] = true
	}
}

func TestEncodeDoesNotContainDSNFields(t *testing.T) {
	data, err := Encode(Report{SourceDatabase: "legacy", TargetDatabase: "moonbook_admin"})
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "password") || strings.Contains(text, "dsn") {
		t.Fatalf("audit report contains forbidden credential fields: %s", text)
	}
}
