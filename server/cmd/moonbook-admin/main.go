package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/adminbootstrap"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) != 2 || os.Args[1] != "bootstrap" {
		fmt.Fprintln(os.Stderr, "usage: moonbook-admin bootstrap")
		return 2
	}
	dsn := strings.TrimSpace(os.Getenv("MOONBOOK_DATABASE_DSN"))
	username := strings.TrimSpace(os.Getenv("MOONBOOK_ADMIN_USERNAME"))
	password := os.Getenv("MOONBOOK_ADMIN_PASSWORD")
	nickname := strings.TrimSpace(os.Getenv("MOONBOOK_ADMIN_NICKNAME"))
	if dsn == "" || username == "" || password == "" {
		fmt.Fprintln(os.Stderr, "MOONBOOK_DATABASE_DSN, MOONBOOK_ADMIN_USERNAME and MOONBOOK_ADMIN_PASSWORD are required")
		return 2
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database: %v\n", err)
		return 1
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	result, err := adminbootstrap.Bootstrap(ctx, db, adminbootstrap.Options{
		Username: username,
		Password: password,
		Nickname: nickname,
	})
	if err != nil {
		if errors.Is(err, adminbootstrap.ErrAlreadyBootstrapped) {
			fmt.Fprintln(os.Stderr, "administrator bootstrap refused: an administrator already exists")
			return 3
		}
		fmt.Fprintf(os.Stderr, "bootstrap administrator: %v\n", err)
		return 1
	}
	fmt.Printf("administrator_created=true id=%d username=%s must_change_password=true\n", result.ID, result.Username)
	return 0
}
