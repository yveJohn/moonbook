package legacyaudit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type TableMapping struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Stage  string `json:"stage"`
}

type TableReport struct {
	Mapping      TableMapping `json:"mapping"`
	SourceExists bool         `json:"sourceExists"`
	TargetExists bool         `json:"targetExists"`
	SourceRows   int64        `json:"sourceRows"`
	TargetRows   int64        `json:"targetRows"`
	SourceMaxID  *int64       `json:"sourceMaxId,omitempty"`
	TargetMaxID  *int64       `json:"targetMaxId,omitempty"`
	LegacyKeys   int64        `json:"legacyKeys"`
	Errors       []string     `json:"errors,omitempty"`
}

type ObjectReport struct {
	Source string `json:"source"`
	State  string `json:"state"`
	Count  int64  `json:"count"`
	Bytes  int64  `json:"bytes"`
}

type Report struct {
	GeneratedAt     time.Time             `json:"generatedAt"`
	SourceDatabase  string                `json:"sourceDatabase"`
	TargetDatabase  string                `json:"targetDatabase"`
	SourceReadOnly  bool                  `json:"sourceReadOnly"`
	Mappings        []TableReport         `json:"mappings"`
	Checkpoints     []Checkpoint          `json:"checkpoints"`
	MigrationErrors int64                 `json:"migrationErrors"`
	Objects         []ObjectReport        `json:"objects"`
	ObjectIntegrity ObjectIntegrityReport `json:"objectIntegrity"`
	Finance         FinanceReport         `json:"finance"`
	IntegrityErrors []string              `json:"integrityErrors,omitempty"`
}

type FinanceReport struct {
	WalletsChecked   int            `json:"walletsChecked"`
	RechargeOrders   int            `json:"rechargeOrders"`
	PurchaseOrders   int            `json:"purchaseOrders"`
	Callbacks        int            `json:"callbacks"`
	MembershipGrants int            `json:"membershipGrants"`
	Entitlements     int            `json:"entitlements"`
	MismatchCount    int            `json:"mismatchCount"`
	Samples          []FinanceIssue `json:"samples,omitempty"`
}

type FinanceIssue struct {
	Domain      string `json:"domain"`
	Field       string `json:"field"`
	Fingerprint string `json:"fingerprint"`
}

type Checkpoint struct {
	Stage     string `json:"stage"`
	Cursor    string `json:"cursor"`
	Processed int64  `json:"processed"`
	Errors    int64  `json:"errors"`
	Done      bool   `json:"done"`
}

var mappings = []TableMapping{
	{Source: "novel_book", Target: "novel_books", Stage: "novel-books"},
	{Source: "novel_chapter", Target: "novel_chapters", Stage: "novel-chapters"},
	{Source: "novel_crawl_forum_source", Target: "novel_crawl_forum_source", Stage: "novel-crawl-sources"},
	{Source: "novel_crawl_forum_board", Target: "novel_crawl_forum_board", Stage: "novel-crawl-sources"},
	{Source: "novel_crawl_thread_candidate", Target: "novel_crawl_thread_candidate", Stage: "novel-crawl-candidates"},
	{Source: "novel_crawl_import_task", Target: "novel_crawl_import_task", Stage: "novel-crawl-import-tasks"},
	{Source: "novel_crawl_fetch_log", Target: "novel_crawl_fetch_log", Stage: "novel-crawl-fetch-logs"},
	{Source: "novel_txt_import_task", Target: "novel_txt_import_task", Stage: "novel-txt-imports"},
	{Source: "novel_ai_config", Target: "novel_ai_config", Stage: "novel-ai-configs"},
	{Source: "novel_ai_config_model", Target: "novel_ai_config_model", Stage: "novel-ai-configs"},
	{Source: "ai_chapter_clean_task", Target: "novel_chapter_clean_task", Stage: "novel-chapter-clean-tasks"},
	{Source: "ai_chapter_clean_result", Target: "novel_chapter_clean_result", Stage: "novel-chapter-clean-results"},
	{Source: "novel_chapter_summary_task_log", Target: "novel_chapter_summary_task", Stage: "novel-chapter-summary-tasks"},
	{Source: "novel_chapter_summary_config", Target: "novel_chapter_summary_config", Stage: "novel-chapter-summary-config"},
	{Source: "novel_book_profile_config", Target: "novel_book_profile_config", Stage: "novel-book-profile-config"},
	{Source: "novel_book_merge_task", Target: "novel_book_merge_task", Stage: "novel-book-merge-tasks"},
	{Source: "novel_book_merge_source", Target: "novel_book_merge_source", Stage: "novel-book-merge-sources"},
	{Source: "novel_book_merge_chapter", Target: "novel_book_merge_chapter", Stage: "novel-book-merge-chapters"},
	{Source: "novel_book_profile_suggestion", Target: "novel_book_profile_suggestion", Stage: "novel-book-profile-suggestions"},
}

func Mappings() []TableMapping { return append([]TableMapping(nil), mappings...) }

func Build(ctx context.Context, source, target *sql.DB, migration string) (Report, error) {
	if source == nil || target == nil {
		return Report{}, fmt.Errorf("source and target databases are required")
	}
	if err := source.PingContext(ctx); err != nil {
		return Report{}, fmt.Errorf("ping source database: %w", err)
	}
	if err := target.PingContext(ctx); err != nil {
		return Report{}, fmt.Errorf("ping target database: %w", err)
	}
	report := Report{GeneratedAt: time.Now().UTC(), SourceReadOnly: true}
	if err := source.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&report.SourceDatabase); err != nil {
		return Report{}, fmt.Errorf("read source database name: %w", err)
	}
	if err := target.QueryRowContext(ctx, "SELECT current_database()").Scan(&report.TargetDatabase); err != nil {
		return Report{}, fmt.Errorf("read target database name: %w", err)
	}
	for _, mapping := range mappings {
		item := TableReport{Mapping: mapping}
		var err error
		item.SourceExists, err = tableExists(ctx, source, mapping.Source, false)
		if err != nil {
			return Report{}, fmt.Errorf("inspect source table %s: %w", mapping.Source, err)
		}
		item.TargetExists, err = tableExists(ctx, target, mapping.Target, true)
		if err != nil {
			return Report{}, fmt.Errorf("inspect target table %s: %w", mapping.Target, err)
		}
		if item.SourceExists {
			item.SourceRows, item.SourceMaxID, err = tableFacts(ctx, source, mapping.Source, false)
			if err != nil {
				return Report{}, fmt.Errorf("read source table facts %s: %w", mapping.Source, err)
			}
		}
		if item.TargetExists {
			item.TargetRows, item.TargetMaxID, err = tableFacts(ctx, target, mapping.Target, true)
			if err != nil {
				return Report{}, fmt.Errorf("read target table facts %s: %w", mapping.Target, err)
			}
			item.LegacyKeys, err = legacyKeyCount(ctx, target, mapping.Target)
			if err != nil {
				return Report{}, fmt.Errorf("read target legacy keys %s: %w", mapping.Target, err)
			}
		}
		if item.SourceExists && !item.TargetExists {
			item.Errors = append(item.Errors, "target_table_missing")
		}
		if item.TargetExists && item.SourceRows > item.TargetRows && !strings.Contains(mapping.Source, "summary") {
			item.Errors = append(item.Errors, "target_row_count_below_source")
		}
		if item.SourceExists && item.TargetExists && item.SourceMaxID != nil && (item.TargetMaxID == nil || *item.TargetMaxID < *item.SourceMaxID) {
			item.Errors = append(item.Errors, "target_max_id_below_source")
		}
		if item.SourceExists && item.TargetExists && item.SourceRows > 0 && item.LegacyKeys > 0 && item.LegacyKeys < item.SourceRows {
			item.Errors = append(item.Errors, "legacy_key_count_below_source")
		}
		report.Mappings = append(report.Mappings, item)
	}
	var err error
	report.Checkpoints, err = checkpoints(ctx, target, migration)
	if err != nil {
		return Report{}, fmt.Errorf("read migration checkpoints: %w", err)
	}
	checkpointStages := make(map[string]bool, len(report.Checkpoints))
	for _, checkpoint := range report.Checkpoints {
		checkpointStages[checkpoint.Stage] = true
		if checkpoint.Errors > 0 || !checkpoint.Done {
			report.IntegrityErrors = append(report.IntegrityErrors, fmt.Sprintf("checkpoint_incomplete:%s", checkpoint.Stage))
		}
	}
	for stage := range StageAuditContracts() {
		if !checkpointStages[stage] {
			report.IntegrityErrors = append(report.IntegrityErrors, "checkpoint_missing:"+stage)
		}
	}
	if err := target.QueryRowContext(ctx, `SELECT count(*) FROM migration_errors WHERE migration_name=$1`, migration).Scan(&report.MigrationErrors); err != nil {
		return Report{}, fmt.Errorf("read migration errors: %w", err)
	}
	report.Objects, err = objectFacts(ctx, target)
	if err != nil {
		return Report{}, fmt.Errorf("read object facts: %w", err)
	}
	report.IntegrityErrors = append(report.IntegrityErrors, integrity(ctx, target)...)
	sort.Strings(report.IntegrityErrors)
	return report, nil
}

func Encode(report Report) ([]byte, error) { return json.MarshalIndent(report, "", "  ") }

func (report Report) HasFailures() bool {
	if report.MigrationErrors > 0 || len(report.IntegrityErrors) > 0 || report.ObjectIntegrity.IssueCount > 0 || report.Finance.MismatchCount > 0 {
		return true
	}
	for _, mapping := range report.Mappings {
		if len(mapping.Errors) > 0 {
			return true
		}
	}
	return false
}

func StageAuditContracts() map[string][]string {
	contracts := map[string][]string{
		"preflight": {"source_read_only"}, "novel-category-dictionary": {"rows", "primary_key", "relations"},
		"legacy-book-category": {"rows", "primary_key", "relations"}, "novel-book-author": {"rows", "primary_key", "relations"},
		"legacy-book_author": {"rows", "primary_key", "relations"}, "legacy-author": {"rows", "primary_key", "relations"},
		"novel-books": {"rows", "primary_key", "relations"}, "novel-book-sub-categories": {"rows", "primary_key", "relations"},
		"novel-book-covers": {"objects"}, "novel-chapters": {"rows", "primary_key", "objects"},
		"novel-crawl-sources": {"rows", "primary_key", "relations"}, "novel-crawl-candidates": {"rows", "primary_key", "relations"},
		"novel-crawl-import-tasks": {"rows", "primary_key", "relations"}, "novel-crawl-fetch-logs": {"rows", "primary_key", "relations"},
		"novel-txt-imports": {"rows", "primary_key", "objects"}, "novel-ai-configs": {"rows", "primary_key", "relations"},
		"novel-chapter-clean-tasks": {"rows", "primary_key", "relations"}, "novel-chapter-clean-results": {"rows", "primary_key", "objects"},
		"novel-chapter-summary-config": {"rows", "primary_key", "relations"}, "novel-chapter-summary-tasks": {"rows", "primary_key", "relations"},
		"novel-book-profile-config": {"rows", "primary_key", "relations"}, "novel-book-profile-suggestions": {"rows", "primary_key", "relations"},
		"novel-book-merge-tasks": {"rows", "primary_key", "relations"}, "novel-book-merge-sources": {"rows", "primary_key", "relations"},
		"novel-book-merge-chapters": {"rows", "primary_key", "objects"}, "novel-reader-seo": {"rows", "primary_key"},
		"reader-identity": {"rows", "primary_key", "relations"}, "reader-commerce": {"rows", "primary_key", "relations"},
		"reader-finance": {"finance_full"}, "reader-activity": {"rows", "primary_key", "relations"},
	}
	return contracts
}

func tableExists(ctx context.Context, db *sql.DB, table string, postgres bool) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?)`
	if postgres {
		query = `SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema=current_schema() AND table_name=$1)`
	}
	var exists bool
	err := db.QueryRowContext(ctx, query, table).Scan(&exists)
	return exists, err
}

func tableFacts(ctx context.Context, db *sql.DB, table string, postgres bool) (int64, *int64, error) {
	quote := "`"
	if postgres {
		quote = `"`
	}
	var rows int64
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM "+quote+table+quote).Scan(&rows); err != nil {
		return 0, nil, err
	}
	var max sql.NullInt64
	if err := db.QueryRowContext(ctx, "SELECT max(id) FROM "+quote+table+quote).Scan(&max); err != nil {
		return 0, nil, err
	}
	if !max.Valid {
		return rows, nil, nil
	}
	value := max.Int64
	return rows, &value, nil
}

func legacyKeyCount(ctx context.Context, db *sql.DB, table string) (int64, error) {
	var count int64
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=$1 AND column_name='legacy_source_key'`, table).Scan(&count); err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, nil
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM `+`"`+table+`" WHERE legacy_source_key IS NOT NULL`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func checkpoints(ctx context.Context, db *sql.DB, migration string) ([]Checkpoint, error) {
	rows, err := db.QueryContext(ctx, `SELECT stage,COALESCE(cursor_value,''),processed_count,error_count,COALESCE((metadata->>'done')::boolean,false) FROM migration_checkpoints WHERE migration_name=$1 ORDER BY stage`, migration)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Checkpoint
	for rows.Next() {
		var c Checkpoint
		if err := rows.Scan(&c.Stage, &c.Cursor, &c.Processed, &c.Errors, &c.Done); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func objectFacts(ctx context.Context, db *sql.DB) ([]ObjectReport, error) {
	rows, err := db.QueryContext(ctx, `SELECT source,state,count(*),COALESCE(sum(byte_size),0) FROM novel_objects GROUP BY source,state ORDER BY source,state`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ObjectReport
	for rows.Next() {
		var item ObjectReport
		if err := rows.Scan(&item.Source, &item.State, &item.Count, &item.Bytes); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func integrity(ctx context.Context, db *sql.DB) []string {
	checks := []struct{ name, query string }{
		{"crawl_board_missing_source", `SELECT count(*) FROM novel_crawl_forum_board b LEFT JOIN novel_crawl_forum_source s ON s.id=b.source_id WHERE s.id IS NULL`},
		{"crawl_candidate_missing_source", `SELECT count(*) FROM novel_crawl_thread_candidate c LEFT JOIN novel_crawl_forum_source s ON s.id=c.source_id WHERE s.id IS NULL`},
		{"crawl_candidate_missing_board", `SELECT count(*) FROM novel_crawl_thread_candidate c LEFT JOIN novel_crawl_forum_board b ON b.id=c.board_id WHERE b.id IS NULL`},
		{"crawl_import_missing_source", `SELECT count(*) FROM novel_crawl_import_task i LEFT JOIN novel_crawl_forum_source s ON s.id=i.source_id WHERE i.source_id IS NOT NULL AND s.id IS NULL`},
		{"crawl_import_missing_board", `SELECT count(*) FROM novel_crawl_import_task i LEFT JOIN novel_crawl_forum_board b ON b.id=i.board_id WHERE i.board_id IS NOT NULL AND b.id IS NULL`},
		{"crawl_import_missing_candidate", `SELECT count(*) FROM novel_crawl_import_task i LEFT JOIN novel_crawl_thread_candidate c ON c.id=i.candidate_id WHERE i.candidate_id IS NOT NULL AND c.id IS NULL`},
		{"txt_import_missing_book", `SELECT count(*) FROM novel_txt_import_task t LEFT JOIN novel_books b ON b.id=t.target_book_id WHERE b.id IS NULL`},
		{"ai_model_missing_config", `SELECT count(*) FROM novel_ai_config_model m LEFT JOIN novel_ai_config c ON c.id=m.ai_config_id WHERE c.id IS NULL`},
		{"clean_task_missing_book", `SELECT count(*) FROM novel_chapter_clean_task t LEFT JOIN novel_books b ON b.id=t.book_id WHERE b.id IS NULL`},
		{"clean_result_missing_task", `SELECT count(*) FROM novel_chapter_clean_result r LEFT JOIN novel_chapter_clean_task t ON t.id=r.task_id WHERE t.id IS NULL`},
		{"clean_result_missing_book", `SELECT count(*) FROM novel_chapter_clean_result r LEFT JOIN novel_books b ON b.id=r.book_id WHERE b.id IS NULL`},
		{"clean_result_missing_chapter", `SELECT count(*) FROM novel_chapter_clean_result r LEFT JOIN novel_chapters c ON c.id=r.chapter_id WHERE c.id IS NULL`},
		{"clean_result_missing_object", `SELECT count(*) FROM novel_chapter_clean_result WHERE active AND status='success' AND cleaned_object_id IS NULL`},
		{"merge_source_missing_task", `SELECT count(*) FROM novel_book_merge_source s LEFT JOIN novel_book_merge_task t ON t.id=s.task_id WHERE t.id IS NULL`},
		{"merge_chapter_missing_task", `SELECT count(*) FROM novel_book_merge_chapter c LEFT JOIN novel_book_merge_task t ON t.id=c.task_id WHERE t.id IS NULL`},
		{"merge_chapter_missing_source_book", `SELECT count(*) FROM novel_book_merge_chapter c LEFT JOIN novel_books b ON b.id=c.source_book_id WHERE b.id IS NULL`},
		{"merge_chapter_missing_target_book", `SELECT count(*) FROM novel_book_merge_chapter c LEFT JOIN novel_books b ON b.id=c.target_book_id WHERE c.target_book_id IS NOT NULL AND b.id IS NULL`},
		{"profile_suggestion_missing_book", `SELECT count(*) FROM novel_book_profile_suggestion s LEFT JOIN novel_books b ON b.id=s.book_id WHERE b.id IS NULL`},
		{"profile_pending_duplicate", `SELECT count(*)-count(DISTINCT book_id) FROM novel_book_profile_suggestion WHERE status IN ('running','pending')`},
	}
	var out []string
	for _, check := range checks {
		var count int64
		if err := db.QueryRowContext(ctx, check.query).Scan(&count); err != nil {
			out = append(out, "check_query_failed:"+check.name)
		} else if count > 0 {
			out = append(out, fmt.Sprintf("%s=%d", check.name, count))
		}
	}
	sort.Strings(out)
	return out
}

func AuditTargetIntegrity(ctx context.Context, db *sql.DB) []string {
	if db == nil {
		return []string{"target_database_missing"}
	}
	return integrity(ctx, db)
}
