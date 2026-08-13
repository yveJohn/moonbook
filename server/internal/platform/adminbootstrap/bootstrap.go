package adminbootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const bootstrapLockID int64 = 6648019266281988081

var ErrAlreadyBootstrapped = errors.New("an administrator already exists")

type Options struct {
	Username string
	Password string
	Nickname string
}

type Result struct {
	ID       int64
	Username string
}

func Bootstrap(ctx context.Context, db *sql.DB, options Options) (Result, error) {
	options.Username = strings.TrimSpace(options.Username)
	options.Nickname = strings.TrimSpace(options.Nickname)
	if options.Username == "" {
		return Result{}, errors.New("username is required")
	}
	if strings.ContainsAny(options.Username, "\r\n\t ") {
		return Result{}, errors.New("username must not contain whitespace")
	}
	if len(options.Password) < 12 {
		return Result{}, errors.New("password must contain at least 12 characters")
	}
	if options.Nickname == "" {
		options.Nickname = options.Username
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(options.Password), bcrypt.DefaultCost)
	if err != nil {
		return Result{}, fmt.Errorf("hash password: %w", err)
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return Result{}, fmt.Errorf("begin bootstrap transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", bootstrapLockID); err != nil {
		return Result{}, fmt.Errorf("lock bootstrap: %w", err)
	}

	var userCount int64
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sys_users").Scan(&userCount); err != nil {
		return Result{}, fmt.Errorf("count administrators: %w", err)
	}
	if userCount != 0 {
		return Result{}, ErrAlreadyBootstrapped
	}
	var authorityCount int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sys_authorities WHERE authority_id = 888").Scan(&authorityCount); err != nil {
		return Result{}, fmt.Errorf("check administrator authority: %w", err)
	}
	if authorityCount != 1 {
		return Result{}, errors.New("administrator authority 888 is missing; apply migrations first")
	}

	now := time.Now().UTC()
	var id int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO sys_users
			(created_at, updated_at, uuid, username, password, nick_name, authority_id, enable, password_updated_at, must_change_password)
		VALUES ($1, $1, $2, $3, $4, $5, 888, 1, $1, true)
		RETURNING id`, now, uuid.NewString(), options.Username, string(hash), options.Nickname).Scan(&id)
	if err != nil {
		return Result{}, fmt.Errorf("create administrator: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sys_user_authority (sys_user_id, sys_authority_authority_id)
		SELECT $1, authority_id FROM sys_authorities`, id); err != nil {
		return Result{}, fmt.Errorf("assign administrator authorities: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("commit bootstrap transaction: %w", err)
	}
	return Result{ID: id, Username: options.Username}, nil
}
