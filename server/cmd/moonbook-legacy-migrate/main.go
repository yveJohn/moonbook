package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/legacymigrate"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() { os.Exit(run()) }

func run() int {
	flag.Parse()
	if flag.NArg() != 1 || flag.Arg(0) != "preflight" {
		fmt.Fprintln(os.Stderr, "usage: moonbook-legacy-migrate preflight")
		return 2
	}
	sourceDSN := strings.TrimSpace(os.Getenv("MOONBOOK_LEGACY_MYSQL_DSN"))
	targetDSN := strings.TrimSpace(os.Getenv("MOONBOOK_DATABASE_DSN"))
	if sourceDSN == "" || targetDSN == "" {
		fmt.Fprintln(os.Stderr, "MOONBOOK_LEGACY_MYSQL_DSN and MOONBOOK_DATABASE_DSN are required")
		return 2
	}
	batchSize := 1000
	if raw := strings.TrimSpace(os.Getenv("MOONBOOK_MIGRATION_BATCH_SIZE")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			fmt.Fprintln(os.Stderr, "MOONBOOK_MIGRATION_BATCH_SIZE must be an integer")
			return 2
		}
		batchSize = value
	}
	source, err := sql.Open("mysql", sourceDSN)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open legacy MySQL:", err)
		return 1
	}
	defer source.Close()
	target, err := sql.Open("pgx", targetDSN)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open PostgreSQL:", err)
		return 1
	}
	defer target.Close()
	runner, err := legacymigrate.NewRunner(source, target, "moonbook-v1", batchSize)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := runner.Run(ctx, legacymigrate.PreflightStage{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("migration=moonbook-v1 stage=preflight status=complete")
	return 0
}
