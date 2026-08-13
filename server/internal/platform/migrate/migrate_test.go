package migrate

import (
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestEmbeddedMigrationsLoad(t *testing.T) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no embedded migrations")
	}
	db, err := sql.Open("pgx", "postgres://unused")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	provider, err := NewProvider(db)
	if err != nil {
		t.Fatalf("load embedded migrations: %v", err)
	}
	if provider == nil {
		t.Fatal("nil migration provider")
	}
}
