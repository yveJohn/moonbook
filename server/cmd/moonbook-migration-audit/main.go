package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/legacyaudit"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/legacymigrate"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() { os.Exit(run()) }

func run() int {
	flag.Parse()
	if flag.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "usage: moonbook-migration-audit [migration-name]")
		return 2
	}
	migration := "moonbook-v1"
	if flag.NArg() == 1 && strings.TrimSpace(flag.Arg(0)) != "" {
		migration = flag.Arg(0)
	}
	sourceDSN := strings.TrimSpace(os.Getenv("MOONBOOK_LEGACY_MYSQL_DSN"))
	targetDSN := strings.TrimSpace(os.Getenv("MOONBOOK_DATABASE_DSN"))
	if sourceDSN == "" || targetDSN == "" {
		fmt.Fprintln(os.Stderr, "MOONBOOK_LEGACY_MYSQL_DSN and MOONBOOK_DATABASE_DSN are required")
		return 2
	}
	source, err := sql.Open("mysql", sourceDSN)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open source:", err)
		return 1
	}
	defer source.Close()
	target, err := sql.Open("pgx", targetDSN)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open target:", err)
		return 1
	}
	defer target.Close()
	timeout := 10 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("MOONBOOK_AUDIT_TIMEOUT")); raw != "" {
		if parsed, e := time.ParseDuration(raw); e == nil && parsed >= time.Minute && parsed <= 24*time.Hour {
			timeout = parsed
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := legacymigrate.VerifySourceReadOnly(ctx, source); err != nil {
		fmt.Fprintln(os.Stderr, "source read-only check:", err)
		return 1
	}
	report, err := legacyaudit.Build(ctx, source, target, migration)
	if err != nil {
		fmt.Fprintln(os.Stderr, "build audit:", err)
		return 1
	}
	data, err := legacyaudit.Encode(report)
	if err != nil {
		fmt.Fprintln(os.Stderr, "encode audit:", err)
		return 1
	}
	fmt.Println(string(data))
	if len(report.IntegrityErrors) > 0 {
		return 1
	}
	return 0
}
