package migrate

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

const versionTable = "moonbook_schema_version"

type Status struct {
	Current int64
	Target  int64
	Pending bool
}

func NewProvider(db *sql.DB) (*goose.Provider, error) {
	migrations, err := fs.Sub(migrationFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("open embedded migrations: %w", err)
	}
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, fmt.Errorf("create migration lock: %w", err)
	}
	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		migrations,
		goose.WithTableName(versionTable),
		goose.WithSessionLocker(locker),
	)
	if err != nil {
		return nil, fmt.Errorf("create migration provider: %w", err)
	}
	return provider, nil
}

func Up(ctx context.Context, db *sql.DB) ([]*goose.MigrationResult, error) {
	provider, err := NewProvider(db)
	if err != nil {
		return nil, err
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return nil, fmt.Errorf("apply migrations: %w", err)
	}
	return results, nil
}

func CurrentStatus(ctx context.Context, db *sql.DB) (Status, error) {
	provider, err := NewProvider(db)
	if err != nil {
		return Status{}, err
	}
	current, target, err := provider.GetVersions(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("read migration versions: %w", err)
	}
	pending, err := provider.HasPending(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("check pending migrations: %w", err)
	}
	return Status{Current: current, Target: target, Pending: pending}, nil
}
