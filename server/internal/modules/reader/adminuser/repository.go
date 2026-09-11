package adminuser

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

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

func (r SQLRepository) ListOperations(ctx context.Context, id int64, p, n int) ([]Operation, int64, error) {
	if id <= 0 {
		return nil, 0, errors.New("invalid reader id")
	}
	readerID := strconv.FormatInt(id, 10)
	where := ` WHERE r.deleted_at IS NULL AND (r.path LIKE '%/reader/users/' || $1 || '/%' OR r.path LIKE '%/reader/wallets/' || $1 || '/%')`
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM sys_operation_records r`+where, readerID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, `
SELECT r.id::text,
       COALESCE(r.created_at::text, ''),
       COALESCE(r.method, ''),
       COALESCE(r.path, ''),
       COALESCE(r.status, 0),
       COALESCE(r.body, ''),
       COALESCE(r.error_message, ''),
       COALESCE(r.user_id::text, ''),
       COALESCE(u.username, ''),
       COALESCE(u.nick_name, '')
FROM sys_operation_records r
LEFT JOIN sys_users u ON u.id = r.user_id AND u.deleted_at IS NULL`+where+`
ORDER BY r.created_at DESC, r.id DESC
LIMIT $2 OFFSET $3`, readerID, n, (p-1)*n)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]Operation, 0)
	for rows.Next() {
		var op Operation
		if err := rows.Scan(&op.ID, &op.CreatedAt, &op.Method, &op.Path, &op.Status, &op.Body, &op.ErrorMessage, &op.OperatorID, &op.OperatorUsername, &op.OperatorNickname); err != nil {
			return nil, 0, err
		}
		items = append(items, op)
	}
	return items, total, rows.Err()
}
