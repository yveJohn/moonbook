package adminuser

import (
	"context"
	"database/sql"
	"errors"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
	"golang.org/x/crypto/bcrypt"
)

type SQLRepository struct{ DB *sql.DB }

const userSelect = `SELECT id::text,username,nickname,status,password_algorithm,COALESCE(last_login_at::text,''),created_at::text,updated_at::text FROM reader_accounts`

func scan(row interface{ Scan(...any) error }, u *User) error {
	return row.Scan(&u.ID, &u.Username, &u.Nickname, &u.Status, &u.PasswordAlgorithm, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
}

func (r SQLRepository) ResetPassword(ctx context.Context, id int64, password, confirm string) error {
	if id <= 0 || password != confirm || len([]rune(password)) < 6 || len([]rune(password)) > 64 {
		return errors.New("invalid reader password")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status string
	if err = tx.QueryRowContext(ctx, `SELECT status FROM reader_accounts WHERE id=$1 FOR UPDATE`, id).Scan(&status); err != nil {
		return err
	}
	if status == "deleted" {
		return errors.New("reader account deleted")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE reader_accounts SET password_hash=$1,password_algorithm='bcrypt',password_upgraded_at=now(),updated_at=now() WHERE id=$2`, string(hash), id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE reader_sessions SET revoked_at=now() WHERE reader_id=$1 AND revoked_at IS NULL`, id); err != nil {
		return err
	}
	return tx.Commit()
}
func (r SQLRepository) List(ctx context.Context, k, status string, p, n int) ([]User, int64, error) {
	where := ` WHERE ($1='' OR username ILIKE '%'||$1||'%' OR nickname ILIKE '%'||$1||'%') AND ($2='' OR status=$2)`
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM reader_accounts`+where, k, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, userSelect+where+` ORDER BY id DESC LIMIT $3 OFFSET $4`, k, status, n, (p-1)*n)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]User, 0)
	for rows.Next() {
		var u User
		if err := scan(rows, &u); err != nil {
			return nil, 0, err
		}
		items = append(items, u)
	}
	return items, total, rows.Err()
}
func (r SQLRepository) Get(ctx context.Context, id int64) (User, error) {
	var u User
	err := scan(r.DB.QueryRowContext(ctx, userSelect+` WHERE id=$1`, id), &u)
	return u, err
}
func (r SQLRepository) SetStatus(ctx context.Context, id int64, status string) (User, error) {
	if status != "enabled" && status != "disabled" {
		return User{}, errors.New("invalid reader status")
	}
	executor := transaction.Executor(ctx, r.DB)
	if executor == r.DB {
		return User{}, transaction.ErrNoTransaction
	}
	var u User
	if err := scan(executor.QueryRowContext(ctx, userSelect+` WHERE id=$1 FOR UPDATE`, id), &u); err != nil {
		return User{}, err
	}
	if _, err := executor.ExecContext(ctx, `UPDATE reader_accounts SET status=$1,updated_at=now() WHERE id=$2`, status, id); err != nil {
		return User{}, err
	}
	if status != "enabled" {
		if _, err := executor.ExecContext(ctx, `UPDATE reader_sessions SET revoked_at=now() WHERE reader_id=$1 AND revoked_at IS NULL`, id); err != nil {
			return User{}, err
		}
	}
	if err := scan(executor.QueryRowContext(ctx, userSelect+` WHERE id=$1`, id), &u); err != nil {
		return User{}, err
	}
	return u, nil
}
