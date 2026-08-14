package auth

import (
	"context"
	"database/sql"
	"time"
)

// SQLRepository is the PostgreSQL implementation of the narrow auth store.
// It deliberately uses database/sql so Reader does not depend on GVA models.
type SQLRepository struct{ DB *sql.DB }

func (r SQLRepository) FindAccountByUsername(ctx context.Context, username string) (ReaderAccount, error) {
	return r.account(ctx, `SELECT id,username,nickname,password_hash,password_algorithm,status,COALESCE(password_upgraded_at,'epoch') FROM reader_accounts WHERE username=$1`, username)
}
func (r SQLRepository) FindAccountByID(ctx context.Context, id int64) (ReaderAccount, error) {
	return r.account(ctx, `SELECT id,username,nickname,password_hash,password_algorithm,status,COALESCE(password_upgraded_at,'epoch') FROM reader_accounts WHERE id=$1`, id)
}
func (r SQLRepository) account(ctx context.Context, query string, arg any) (ReaderAccount, error) {
	var a ReaderAccount
	err := r.DB.QueryRowContext(ctx, query, arg).Scan(&a.ID, &a.Username, &a.Nickname, &a.PasswordHash, &a.PasswordAlgorithm, &a.Status, &a.PasswordUpgradedAt)
	return a, err
}
func (r SQLRepository) CreateSession(ctx context.Context, readerID int64, expires time.Time) (int64, error) {
	var id int64
	err := r.DB.QueryRowContext(ctx, `INSERT INTO reader_sessions(reader_id,token_digest,expires_at) VALUES($1,repeat('0',64),$2) RETURNING id`, readerID, expires).Scan(&id)
	return id, err
}
func (r SQLRepository) UpgradePasswordAndCreateSession(ctx context.Context, id int64, oldHash, newHash string, expires time.Time) (int64, error) {
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	var n int64
	if e = tx.QueryRowContext(ctx, `UPDATE reader_accounts SET password_hash=$1,password_algorithm='bcrypt',password_upgraded_at=now(),updated_at=now() WHERE id=$2 AND password_hash=$3 RETURNING id`, newHash, id, oldHash).Scan(&n); e != nil {
		return 0, e
	}
	if e = tx.QueryRowContext(ctx, `INSERT INTO reader_sessions(reader_id,token_digest,expires_at) VALUES($1,repeat('0',64),$2) RETURNING id`, id, expires).Scan(&n); e != nil {
		return 0, e
	}
	return n, tx.Commit()
}
func (r SQLRepository) SetSessionDigest(ctx context.Context, id int64, digest string) error {
	_, e := r.DB.ExecContext(ctx, `UPDATE reader_sessions SET token_digest=$1,last_seen_at=now() WHERE id=$2`, digest, id)
	return e
}
func (r SQLRepository) FindSession(ctx context.Context, id, readerID int64, digest string) (ReaderSession, error) {
	var s ReaderSession
	var revoked sql.NullTime
	e := r.DB.QueryRowContext(ctx, `SELECT id,reader_id,token_digest,expires_at,revoked_at FROM reader_sessions WHERE id=$1 AND reader_id=$2 AND token_digest=$3`, id, readerID, digest).Scan(&s.ID, &s.ReaderID, &s.TokenDigest, &s.ExpiresAt, &revoked)
	if revoked.Valid {
		s.RevokedAt = &revoked.Time
	}
	return s, e
}
func (r SQLRepository) RevokeSession(ctx context.Context, id, readerID int64, at time.Time) error {
	_, e := r.DB.ExecContext(ctx, `UPDATE reader_sessions SET revoked_at=$1 WHERE id=$2 AND reader_id=$3`, at, id, readerID)
	return e
}
func (r SQLRepository) ReplacePasswordAndRevokeSessions(ctx context.Context, id int64, oldHash, newHash string, except int64, at time.Time) error {
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var n int64
	if e = tx.QueryRowContext(ctx, `UPDATE reader_accounts SET password_hash=$1,password_algorithm='bcrypt',updated_at=now() WHERE id=$2 AND password_hash=$3 RETURNING id`, newHash, id, oldHash).Scan(&n); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE reader_sessions SET revoked_at=$1 WHERE reader_id=$2 AND id<>$3 AND revoked_at IS NULL`, at, id, except); e != nil {
		return e
	}
	return tx.Commit()
}
