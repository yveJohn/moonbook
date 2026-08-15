package legacymigrate

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

type BatchResult struct {
	NextCursor string
	Processed  int64
	Errors     []RecordError
	Done       bool
	Metadata   map[string]any
}

type RecordError struct {
	SourceTable string
	SourceID    string
	Code        string
	Message     string
	Retryable   bool
}

type Stage interface {
	Name() string
	RunBatch(context.Context, *sql.DB, *sql.Tx, string, int) (BatchResult, error)
}

type Runner struct {
	source       *sql.DB
	target       *sql.DB
	migration    string
	batchSize    int
	verifySource func(context.Context, *sql.DB) error
}

func NewRunner(source, target *sql.DB, migration string, batchSize int) (*Runner, error) {
	migration = strings.TrimSpace(migration)
	if source == nil || target == nil {
		return nil, errors.New("source and target databases are required")
	}
	if migration == "" || len(migration) > 128 {
		return nil, errors.New("migration name must contain 1 to 128 characters")
	}
	if batchSize < 1 || batchSize > 10000 {
		return nil, errors.New("batch size must be between 1 and 10000")
	}
	return &Runner{source: source, target: target, migration: migration, batchSize: batchSize, verifySource: VerifySourceReadOnly}, nil
}

func VerifySourceReadOnly(ctx context.Context, source *sql.DB) error {
	var readOnly, superReadOnly int
	var characterSet string
	if err := source.QueryRowContext(ctx, "SELECT @@global.read_only, @@global.super_read_only, @@character_set_connection").Scan(&readOnly, &superReadOnly, &characterSet); err != nil {
		return fmt.Errorf("verify legacy MySQL read-only mode: %w", err)
	}
	if readOnly != 1 && superReadOnly != 1 {
		return errors.New("legacy MySQL is writable; migration requires read_only or super_read_only")
	}
	if !strings.EqualFold(characterSet, "utf8mb4") {
		return fmt.Errorf("legacy MySQL connection charset is %s; migration requires utf8mb4", characterSet)
	}
	return nil
}

func (runner *Runner) Run(ctx context.Context, stages ...Stage) error {
	if err := runner.verifySource(ctx, runner.source); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, stage := range stages {
		if stage == nil {
			return errors.New("migration stage is nil")
		}
		name := strings.TrimSpace(stage.Name())
		if name == "" || len(name) > 128 || seen[name] {
			return fmt.Errorf("invalid or duplicate migration stage %q", name)
		}
		seen[name] = true
		if err := runner.runStage(ctx, name, stage); err != nil {
			return fmt.Errorf("run stage %s: %w", name, err)
		}
	}
	return nil
}

func (runner *Runner) runStage(ctx context.Context, name string, stage Stage) error {
	cursor, done, err := runner.checkpoint(ctx, name)
	if err != nil || done {
		return err
	}
	for {
		tx, err := runner.target.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
		if err != nil {
			return fmt.Errorf("begin batch transaction: %w", err)
		}
		var result BatchResult
		err = transaction.WithExisting(ctx, tx, func(txCtx context.Context) error {
			var runErr error
			result, runErr = stage.RunBatch(txCtx, runner.source, tx, cursor, runner.batchSize)
			if runErr != nil {
				return runErr
			}
			if result.Processed < 0 || int64(len(result.Errors)) > result.Processed {
				return errors.New("stage returned invalid batch counters")
			}
			if !result.Done && result.Processed == 0 && result.NextCursor == cursor {
				return errors.New("stage made no progress")
			}
			return runner.persistBatch(txCtx, tx, name, result)
		})
		if err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit batch transaction: %w", err)
		}
		cursor = result.NextCursor
		if result.Done {
			return nil
		}
	}
}

func (runner *Runner) checkpoint(ctx context.Context, stage string) (string, bool, error) {
	var cursor sql.NullString
	var metadata []byte
	err := runner.target.QueryRowContext(ctx, `
		SELECT cursor_value, metadata FROM migration_checkpoints
		WHERE migration_name=$1 AND stage=$2`, runner.migration, stage).Scan(&cursor, &metadata)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read migration checkpoint: %w", err)
	}
	var state struct {
		Done bool `json:"done"`
	}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &state); err != nil {
			return "", false, fmt.Errorf("decode migration checkpoint metadata: %w", err)
		}
	}
	return cursor.String, state.Done, nil
}

func (runner *Runner) persistBatch(ctx context.Context, tx *sql.Tx, stage string, result BatchResult) error {
	for _, recordError := range result.Errors {
		recordError.SourceTable = strings.TrimSpace(recordError.SourceTable)
		recordError.SourceID = strings.TrimSpace(recordError.SourceID)
		recordError.Code = strings.TrimSpace(recordError.Code)
		recordError.Message = strings.TrimSpace(recordError.Message)
		if recordError.Code == "" || recordError.Message == "" {
			return errors.New("migration record errors require code and message")
		}
		if len(recordError.SourceTable) > 128 || len(recordError.SourceID) > 191 || len(recordError.Code) > 64 {
			return errors.New("migration record error identity exceeds database limits")
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO migration_errors
				(migration_name,stage,source_table,source_id,error_code,error_message,retryable)
			VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6,$7)`,
			runner.migration, stage, recordError.SourceTable, recordError.SourceID,
			recordError.Code, recordError.Message, recordError.Retryable); err != nil {
			return fmt.Errorf("record migration error: %w", err)
		}
	}
	metadata := result.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["done"] = result.Done
	metadata["updatedAt"] = time.Now().UTC().Format(time.RFC3339Nano)
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("encode checkpoint metadata: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO migration_checkpoints
			(migration_name,stage,cursor_value,processed_count,error_count,metadata)
		VALUES ($1,$2,NULLIF($3,''),$4,$5,$6)
		ON CONFLICT (migration_name,stage) DO UPDATE
		SET cursor_value=EXCLUDED.cursor_value,
			processed_count=migration_checkpoints.processed_count+EXCLUDED.processed_count,
			error_count=migration_checkpoints.error_count+EXCLUDED.error_count,
			metadata=EXCLUDED.metadata,updated_at=now()`,
		runner.migration, stage, result.NextCursor, result.Processed, len(result.Errors), encoded); err != nil {
		return fmt.Errorf("update migration checkpoint: %w", err)
	}
	return nil
}
