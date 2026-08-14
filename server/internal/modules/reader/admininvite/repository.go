package admininvite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

type SQLRepository struct{ DB *sql.DB }

const inviteSelect = `SELECT id::text,COALESCE(inviter_reader_id::text,''),code,status,COALESCE(max_use_count::text,''),used_count::text,COALESCE(expires_at::text,''),created_at::text FROM reader_invite_codes`

func scan(row interface{ Scan(...any) error }, v *InviteCode) error {
	return row.Scan(&v.ID, &v.InviterReaderID, &v.Code, &v.Status, &v.MaxUseCount, &v.UsedCount, &v.ExpiresAt, &v.CreatedAt)
}
func (r SQLRepository) List(ctx context.Context, k string, p, n int) ([]InviteCode, int64, error) {
	where := ` WHERE ($1='' OR code ILIKE '%'||$1||'%')`
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM reader_invite_codes`+where, k).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, inviteSelect+where+` ORDER BY id DESC LIMIT $2 OFFSET $3`, k, n, (p-1)*n)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]InviteCode, 0)
	for rows.Next() {
		var v InviteCode
		if err := scan(rows, &v); err != nil {
			return nil, 0, err
		}
		items = append(items, v)
	}
	return items, total, rows.Err()
}
func (r SQLRepository) Create(ctx context.Context, in CreateInput) (InviteCode, error) {
	code := strings.TrimSpace(in.Code)
	if code == "" {
		b := make([]byte, 6)
		if _, err := rand.Read(b); err != nil {
			return InviteCode{}, err
		}
		code = "MB" + strings.ToUpper(hex.EncodeToString(b))
	}
	if len([]rune(code)) > 64 {
		return InviteCode{}, errors.New("invalid invite code")
	}
	var max any
	if strings.TrimSpace(in.MaxUseCount) != "" {
		var n int64
		if _, err := fmt.Sscan(in.MaxUseCount, &n); err != nil || n <= 0 {
			return InviteCode{}, errors.New("invalid max use count")
		}
		max = n
	}
	var exp any
	if strings.TrimSpace(in.ExpiresAt) != "" {
		exp = strings.TrimSpace(in.ExpiresAt)
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return InviteCode{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('reader-invite-id',0))`); err != nil {
		return InviteCode{}, err
	}
	var v InviteCode
	err = scan(tx.QueryRowContext(ctx, `INSERT INTO reader_invite_codes(id,code,status,max_use_count,expires_at) VALUES((SELECT COALESCE(max(id),0)+1 FROM reader_invite_codes),$1,'enabled',$2,$3) RETURNING `+`id::text,'',code,status,COALESCE(max_use_count::text,''),used_count::text,COALESCE(expires_at::text,''),created_at::text`, code, max, exp), &v)
	if err != nil {
		return InviteCode{}, err
	}
	return v, tx.Commit()
}
func (r SQLRepository) SetStatus(ctx context.Context, id int64, status string) (InviteCode, error) {
	if status != "enabled" && status != "disabled" {
		return InviteCode{}, errors.New("invalid invite status")
	}
	var v InviteCode
	err := scan(r.DB.QueryRowContext(ctx, `UPDATE reader_invite_codes SET status=$1,updated_at=now() WHERE id=$2 RETURNING `+`id::text,COALESCE(inviter_reader_id::text,''),code,status,COALESCE(max_use_count::text,''),used_count::text,COALESCE(expires_at::text,''),created_at::text`, status, id), &v)
	return v, err
}
func (r SQLRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM reader_invite_codes WHERE id=$1 AND used_count=0 AND NOT EXISTS(SELECT 1 FROM reader_invite_relations WHERE invite_code_id=$1)`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("invite code used or not found")
	}
	return nil
}
