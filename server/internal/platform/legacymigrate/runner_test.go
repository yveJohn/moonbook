package legacymigrate

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestNewRunnerValidatesConfiguration(t *testing.T) {
	if _, err := NewRunner(nil, nil, "", 0); err == nil {
		t.Fatal("NewRunner returned nil error")
	}
}

type targetStage struct {
	name   string
	marker string
	fail   bool
	calls  int
	target *sql.DB
	bound  bool
}

func (stage *targetStage) Name() string { return stage.name }

func (stage *targetStage) RunBatch(ctx context.Context, _ *sql.DB, target *sql.Tx, _ string, _ int) (BatchResult, error) {
	stage.calls++
	stage.bound = transaction.Executor(ctx, stage.target) == target
	if _, err := target.ExecContext(ctx, `
		INSERT INTO migration_errors
			(migration_name,stage,error_code,error_message,retryable)
		VALUES ($1,$2,'TARGET_WRITE','transaction marker',false)`, stage.marker, stage.name); err != nil {
		return BatchResult{}, err
	}
	if stage.fail {
		return BatchResult{}, errors.New("forced stage failure")
	}
	return BatchResult{NextCursor: "done", Processed: 1, Done: true}, nil
}

func TestRunnerPersistsCheckpointAndRollsBackFailedStageWithPostgres(t *testing.T) {
	dsn := os.Getenv("MOONBOOK_MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("MOONBOOK_MIGRATION_TEST_DSN is not configured")
	}
	target, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	migration := "runner-test-" + time.Now().UTC().Format("150405.000000000")
	runner, err := NewRunner(&sql.DB{}, target, migration, 100)
	if err != nil {
		t.Fatal(err)
	}
	runner.verifySource = func(context.Context, *sql.DB) error { return nil }
	success := &targetStage{name: "success", marker: migration, target: target}
	if err := runner.Run(ctx, success); err != nil {
		t.Fatal(err)
	}
	if err := runner.Run(ctx, success); err != nil || success.calls != 1 {
		t.Fatalf("idempotent rerun calls=%d err=%v", success.calls, err)
	}
	if !success.bound {
		t.Fatal("runner did not bind the batch transaction to the stage context")
	}
	var processed int64
	var done bool
	if err := target.QueryRowContext(ctx, `
		SELECT processed_count,(metadata->>'done')::boolean
		FROM migration_checkpoints WHERE migration_name=$1 AND stage='success'`, migration).Scan(&processed, &done); err != nil {
		t.Fatal(err)
	}
	if processed != 1 || !done {
		t.Fatalf("checkpoint processed=%d done=%t", processed, done)
	}

	failure := &targetStage{name: "failure", marker: migration, fail: true, target: target}
	if err := runner.Run(ctx, failure); err == nil {
		t.Fatal("failed stage returned nil error")
	}
	var checkpointCount, successMarkerCount, failureMarkerCount int
	if err := target.QueryRowContext(ctx, "SELECT count(*) FROM migration_checkpoints WHERE migration_name=$1 AND stage='failure'", migration).Scan(&checkpointCount); err != nil {
		t.Fatal(err)
	}
	if err := target.QueryRowContext(ctx, "SELECT count(*) FROM migration_errors WHERE migration_name=$1 AND stage='success'", migration).Scan(&successMarkerCount); err != nil {
		t.Fatal(err)
	}
	if err := target.QueryRowContext(ctx, "SELECT count(*) FROM migration_errors WHERE migration_name=$1 AND stage='failure'", migration).Scan(&failureMarkerCount); err != nil {
		t.Fatal(err)
	}
	if checkpointCount != 0 || successMarkerCount != 1 || failureMarkerCount != 0 {
		t.Fatalf("rollback checkpoint=%d successMarker=%d failureMarker=%d", checkpointCount, successMarkerCount, failureMarkerCount)
	}
}
