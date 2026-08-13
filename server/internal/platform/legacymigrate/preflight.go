package legacymigrate

import (
	"context"
	"database/sql"
)

type PreflightStage struct{}

func (PreflightStage) Name() string { return "preflight" }

func (PreflightStage) RunBatch(ctx context.Context, source *sql.DB, _ *sql.Tx, _ string, _ int) (BatchResult, error) {
	var version, database string
	var tables int64
	err := source.QueryRowContext(ctx, `
		SELECT VERSION(), DATABASE(), count(*)
		FROM information_schema.tables
		WHERE table_schema=DATABASE() AND table_type='BASE TABLE'`).Scan(&version, &database, &tables)
	if err != nil {
		return BatchResult{}, err
	}
	return BatchResult{
		NextCursor: "complete",
		Processed:  tables,
		Done:       true,
		Metadata: map[string]any{
			"sourceDatabase": database,
			"sourceVersion":  version,
			"sourceTables":   tables,
		},
	}, nil
}
