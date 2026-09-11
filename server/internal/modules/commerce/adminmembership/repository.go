package adminmembership

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

type SQLRepository struct{ DB *sql.DB }

const grantSelect = `SELECT id::text,reader_id::text,grant_type,starts_at::text,COALESCE(expires_at::text,''),permanent::text,status,source_type,source_ref,remark,created_at::text FROM commerce_membership_grants`

func scan(row interface{ Scan(...any) error }, v *Grant) error {
	return row.Scan(&v.ID, &v.ReaderID, &v.GrantType, &v.StartsAt, &v.ExpiresAt, &v.Permanent, &v.Status, &v.SourceType, &v.SourceRef, &v.Remark, &v.CreatedAt)
}

func (r SQLRepository) Grant(ctx context.Context, readerID int64, in Input) (Grant, error) {
	executor := transaction.Executor(ctx, r.DB)
	if executor == r.DB {
		return Grant{}, transaction.ErrNoTransaction
	}
	var v Grant
	if err := scan(executor.QueryRowContext(ctx, grantSelect+` WHERE reader_id=$1 AND source_type='admin' AND source_ref=$2`, readerID, in.RequestID), &v); err == nil {
		return v, nil
	} else if err != sql.ErrNoRows {
		return Grant{}, err
	}
	duration := in.DurationDays
	if in.Permanent {
		duration = "0"
	}
	q := `INSERT INTO commerce_membership_grants(reader_id,grant_type,starts_at,expires_at,permanent,status,source_type,source_ref,remark) VALUES($1,'admin',now(),CASE WHEN $2 THEN NULL ELSE now()+($3::bigint::text||' days')::interval END,$2,'active','admin',$4,$5) RETURNING id::text,reader_id::text,grant_type,starts_at::text,COALESCE(expires_at::text,''),permanent::text,status,source_type,source_ref,remark,created_at::text`
	if err := scan(executor.QueryRowContext(ctx, q, readerID, in.Permanent, duration, in.RequestID, in.Remark), &v); err != nil {
		return Grant{}, err
	}
	return v, nil
}

func (r SQLRepository) MembershipProduct(ctx context.Context, id int64) (MembershipProduct, error) {
	if id <= 0 {
		return MembershipProduct{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "会员商品ID必须是正整数字符串")
	}
	var product MembershipProduct
	var duration sql.NullInt64
	err := r.DB.QueryRowContext(ctx, `SELECT id::text,product_name,duration_days FROM commerce_products WHERE id=$1 AND product_type='membership'`, id).Scan(&product.ID, &product.Name, &duration)
	if err == sql.ErrNoRows {
		return MembershipProduct{}, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "会员商品不存在")
	}
	if err != nil {
		return MembershipProduct{}, err
	}
	if duration.Valid {
		days := int(duration.Int64)
		product.DurationDays = &days
	}
	return product, nil
}
