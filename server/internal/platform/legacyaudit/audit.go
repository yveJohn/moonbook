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
	GeneratedAt     time.Time      `json:"generatedAt"`
	SourceDatabase  string         `json:"sourceDatabase"`
	TargetDatabase  string         `json:"targetDatabase"`
	SourceReadOnly  bool           `json:"sourceReadOnly"`
	Mappings        []TableReport  `json:"mappings"`
	Checkpoints     []Checkpoint   `json:"checkpoints"`
	MigrationErrors int64          `json:"migrationErrors"`
	Objects         []ObjectReport `json:"objects"`
	IntegrityErrors []string       `json:"integrityErrors,omitempty"`
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
	report := Report{GeneratedAt: time.Now().UTC(), SourceReadOnly: true}
	_ = source.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&report.SourceDatabase)
	_ = target.QueryRowContext(ctx, "SELECT current_database()").Scan(&report.TargetDatabase)
	for _, mapping := range mappings {
		item := TableReport{Mapping: mapping}
		item.SourceExists, _ = tableExists(ctx, source, mapping.Source, false)
		item.TargetExists, _ = tableExists(ctx, target, mapping.Target, true)
		if item.SourceExists {
			item.SourceRows, item.SourceMaxID = tableFacts(ctx, source, mapping.Source, false)
		}
		if item.TargetExists {
			item.TargetRows, item.TargetMaxID = tableFacts(ctx, target, mapping.Target, true)
			item.LegacyKeys = legacyKeyCount(ctx, target, mapping.Target)
		}
		if item.SourceExists && !item.TargetExists {
			item.Errors = append(item.Errors, "target_table_missing")
		}
		if item.TargetExists && item.SourceRows > item.TargetRows && !strings.Contains(mapping.Source, "summary") {
			item.Errors = append(item.Errors, "target_row_count_below_source")
		}
		report.Mappings = append(report.Mappings, item)
	}
	report.Checkpoints = checkpoints(ctx, target, migration)
	_ = target.QueryRowContext(ctx, `SELECT count(*) FROM migration_errors WHERE migration_name=$1`, migration).Scan(&report.MigrationErrors)
	report.Objects = objectFacts(ctx, target)
	report.IntegrityErrors = integrity(ctx, target)
	return report, nil
}

func Encode(report Report) ([]byte, error) { return json.MarshalIndent(report, "", "  ") }

func tableExists(ctx context.Context, db *sql.DB, table string, postgres bool) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?)`
	if postgres {
		query = `SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema=current_schema() AND table_name=$1)`
	}
	var exists bool
	err := db.QueryRowContext(ctx, query, table).Scan(&exists)
	return exists, err
}

func tableFacts(ctx context.Context, db *sql.DB, table string, postgres bool) (int64, *int64) {
	quote := "`"
	if postgres {
		quote = `"`
	}
	var rows int64
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM "+quote+table+quote).Scan(&rows); err != nil {
		return 0, nil
	}
	var max sql.NullInt64
	if err := db.QueryRowContext(ctx, "SELECT max(id) FROM "+quote+table+quote).Scan(&max); err != nil || !max.Valid {
		return rows, nil
	}
	value := max.Int64
	return rows, &value
}

func legacyKeyCount(ctx context.Context, db *sql.DB, table string) int64 {
	var count int64
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=$1 AND column_name='legacy_source_key'`, table).Scan(&count)
	if count == 0 {
		return 0
	}
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM `+`"`+table+`" WHERE legacy_source_key IS NOT NULL`).Scan(&count)
	return count
}

func checkpoints(ctx context.Context, db *sql.DB, migration string) []Checkpoint {
	rows, err := db.QueryContext(ctx, `SELECT stage,COALESCE(cursor_value,''),processed_count,error_count,COALESCE((metadata->>'done')::boolean,false) FROM migration_checkpoints WHERE migration_name=$1 ORDER BY stage`, migration)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Checkpoint
	for rows.Next() {
		var c Checkpoint
		if rows.Scan(&c.Stage, &c.Cursor, &c.Processed, &c.Errors, &c.Done) == nil {
			out = append(out, c)
		}
	}
	return out
}

func objectFacts(ctx context.Context, db *sql.DB) []ObjectReport {
	rows, err := db.QueryContext(ctx, `SELECT source,state,count(*),COALESCE(sum(byte_size),0) FROM novel_objects GROUP BY source,state ORDER BY source,state`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []ObjectReport
	for rows.Next() {
		var item ObjectReport
		if rows.Scan(&item.Source, &item.State, &item.Count, &item.Bytes) == nil {
			out = append(out, item)
		}
	}
	return out
}

func integrity(ctx context.Context, db *sql.DB) []string {
	checks := []struct{ name, query string }{
		{"clean_result_missing_task", `SELECT count(*) FROM novel_chapter_clean_result r LEFT JOIN novel_chapter_clean_task t ON t.id=r.task_id WHERE t.id IS NULL`},
		{"clean_result_missing_object", `SELECT count(*) FROM novel_chapter_clean_result WHERE active AND status='success' AND cleaned_object_id IS NULL`},
		{"merge_source_missing_task", `SELECT count(*) FROM novel_book_merge_source s LEFT JOIN novel_book_merge_task t ON t.id=s.task_id WHERE t.id IS NULL`},
		{"merge_chapter_missing_task", `SELECT count(*) FROM novel_book_merge_chapter c LEFT JOIN novel_book_merge_task t ON t.id=c.task_id WHERE t.id IS NULL`},
		{"profile_pending_duplicate", `SELECT count(*)-count(DISTINCT book_id) FROM novel_book_profile_suggestion WHERE status IN ('running','pending')`},
	}
	var out []string
	for _, check := range checks {
		var count int64
		if err := db.QueryRowContext(ctx, check.query).Scan(&count); err == nil && count > 0 {
			out = append(out, fmt.Sprintf("%s=%d", check.name, count))
		}
	}
	sort.Strings(out)
	return out
}
