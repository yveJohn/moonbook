package adminmembership

import (
	"context"
	"database/sql"
)

type SQLRepository struct{ DB *sql.DB }

const grantSelect = `SELECT id::text,reader_id::text,grant_type,starts_at::text,COALESCE(expires_at::text,''),permanent::text,status,source_type,source_ref,remark,created_at::text FROM commerce_membership_grants`

func scan(row interface{ Scan(...any) error }, v *Grant) error {
	return row.Scan(&v.ID, &v.ReaderID, &v.GrantType, &v.StartsAt, &v.ExpiresAt, &v.Permanent, &v.Status, &v.SourceType, &v.SourceRef, &v.Remark, &v.CreatedAt)
}

func (r SQLRepository) Grant(ctx context.Context, readerID int64, in Input) (Grant, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return Grant{}, err
	}
	defer tx.Rollback()
	var status string
	if err = tx.QueryRowContext(ctx, `SELECT status FROM reader_accounts WHERE id=$1 FOR UPDATE`, readerID).Scan(&status); err != nil {
		return Grant{}, err
	}
	if status == "deleted" {
		return Grant{}, sql.ErrNoRows
	}
	var v Grant
	if err = scan(tx.QueryRowContext(ctx, grantSelect+` WHERE reader_id=$1 AND source_type='admin' AND source_ref=$2`, readerID, in.RequestID), &v); err == nil {
		return v, nil
	} else if err != sql.ErrNoRows {
		return Grant{}, err
	}
	duration := in.DurationDays
	if in.Permanent {
		duration = "0"
	}
	q := `INSERT INTO commerce_membership_grants(reader_id,grant_type,starts_at,expires_at,permanent,status,source_type,source_ref,remark) VALUES($1,'admin',now(),CASE WHEN $2 THEN NULL ELSE now()+($3::bigint::text||' days')::interval END,$2,'active','admin',$4,$5) RETURNING id::text,reader_id::text,grant_type,starts_at::text,COALESCE(expires_at::text,''),permanent::text,status,source_type,source_ref,remark,created_at::text`
	if err = scan(tx.QueryRowContext(ctx, q, readerID, in.Permanent, duration, in.RequestID, in.Remark), &v); err != nil {
		return Grant{}, err
	}
	if err = tx.Commit(); err != nil {
		return Grant{}, err
	}
	return v, nil
}
