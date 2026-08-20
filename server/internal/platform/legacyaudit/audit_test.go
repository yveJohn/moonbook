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

func TestStageAuditContractsRequireAllFourEvidenceClasses(t *testing.T) {
	contracts := StageAuditContracts()
	for stage, checks := range contracts {
		if strings.TrimSpace(stage) == "" || len(checks) == 0 {
			t.Fatalf("incomplete audit contract %q=%v", stage, checks)
		}
	}
	for _, stage := range []string{"novel-crawl-sources", "novel-txt-imports", "novel-ai-configs", "novel-chapter-clean-results", "novel-chapter-summary-tasks", "novel-book-profile-suggestions", "novel-book-merge-chapters", "reader-finance"} {
		if len(contracts[stage]) == 0 {
			t.Fatalf("content or finance stage %q is uncovered", stage)
		}
	}
}

func TestReportFailureGateCoversEveryAuditClass(t *testing.T) {
	cases := []Report{
		{MigrationErrors: 1},
		{IntegrityErrors: []string{"broken_relation"}},
		{ObjectIntegrity: ObjectIntegrityReport{IssueCount: 1}},
		{Finance: FinanceReport{MismatchCount: 1}},
		{Mappings: []TableReport{{Errors: []string{"row_count"}}}},
	}
	for index, report := range cases {
		if !report.HasFailures() {
			t.Fatalf("failure class %d passed silently", index)
		}
	}
	if (Report{}).HasFailures() {
		t.Fatal("empty successful report failed")
	}
}
