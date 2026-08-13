package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/migrate"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	os.Exit(run())
}

func run() int {
	flag.Parse()
	action := "status"
	if flag.NArg() > 0 {
		action = strings.ToLower(flag.Arg(0))
	}
	dsn := strings.TrimSpace(os.Getenv("MOONBOOK_DATABASE_DSN"))
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "MOONBOOK_DATABASE_DSN is required")
		return 2
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database: %v\n", err)
		return 1
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "connect database: %v\n", err)
		return 1
	}

	switch action {
	case "up":
		results, err := migrate.Up(ctx, db)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("applied=%d\n", len(results))
	case "status":
		status, err := migrate.CurrentStatus(ctx, db)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("current=%d target=%d pending=%t\n", status.Current, status.Target, status.Pending)
	default:
		fmt.Fprintf(os.Stderr, "unsupported action %q; use up or status\n", action)
		return 2
	}
	return 0
}
