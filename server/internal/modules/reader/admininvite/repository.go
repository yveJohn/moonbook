package admininvite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/jackc/pgx/v5/pgconn"
)

type SQLRepository struct{ DB *sql.DB }

const inviteSelect = `SELECT id::text,COALESCE(inviter_reader_id::text,''),code,status,COALESCE(max_use_count::text,''),used_count::text,COALESCE(expires_at::text,''),remark,created_at::text FROM reader_invite_codes`

func scan(row interface{ Scan(...any) error }, v *InviteCode) error {
	return row.Scan(&v.ID, &v.InviterReaderID, &v.Code, &v.Status, &v.MaxUseCount, &v.UsedCount, &v.ExpiresAt, &v.Remark, &v.CreatedAt)
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
		return InviteCode{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "邀请码不能超过64个字符")
	}
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "enabled"
	}
	if status != "enabled" && status != "disabled" {
		return InviteCode{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "邀请码状态无效")
	}
	max, err := optionalPositiveInt(in.MaxUseCount)
	if err != nil {
		return InviteCode{}, err
	}
	exp, err := optionalTime(in.ExpiresAt)
	if err != nil {
		return InviteCode{}, err
	}
	remark := strings.TrimSpace(in.Remark)
	if len([]rune(remark)) > 255 {
		return InviteCode{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "邀请码备注不能超过255个字符")
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
	err = scan(tx.QueryRowContext(ctx, `INSERT INTO reader_invite_codes(id,code,status,max_use_count,expires_at,remark) VALUES((SELECT COALESCE(max(id),0)+1 FROM reader_invite_codes),$1,$2,$3,$4,$5) RETURNING `+`id::text,'',code,status,COALESCE(max_use_count::text,''),used_count::text,COALESCE(expires_at::text,''),remark,created_at::text`, code, status, max, exp, remark), &v)
	if err != nil {
		return InviteCode{}, mapWriteError(err)
	}
	return v, tx.Commit()
}
func (r SQLRepository) Update(ctx context.Context, id int64, in UpdateInput) (InviteCode, error) {
	if id <= 0 {
		return InviteCode{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "邀请码ID无效")
	}
	status := strings.TrimSpace(in.Status)
	remark := strings.TrimSpace(in.Remark)
	if (status != "enabled" && status != "disabled") || len([]rune(remark)) > 255 {
		return InviteCode{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "邀请码编辑参数无效")
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return InviteCode{}, err
	}
	defer tx.Rollback()
	var automatic bool
	var usedCount int64
	if err = tx.QueryRowContext(ctx, `SELECT inviter_reader_id IS NOT NULL,used_count FROM reader_invite_codes WHERE id=$1 FOR UPDATE`, id).Scan(&automatic, &usedCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return InviteCode{}, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "邀请码不存在")
		}
		return InviteCode{}, err
	}
	var v InviteCode
	if automatic {
		err = scan(tx.QueryRowContext(ctx, `UPDATE reader_invite_codes SET status=$1,remark=$2,updated_at=now() WHERE id=$3 RETURNING `+`id::text,COALESCE(inviter_reader_id::text,''),code,status,COALESCE(max_use_count::text,''),used_count::text,COALESCE(expires_at::text,''),remark,created_at::text`, status, remark, id), &v)
	} else {
		code := strings.TrimSpace(in.Code)
		if code == "" || len([]rune(code)) > 64 {
			return InviteCode{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "邀请码不能为空且不能超过64个字符")
		}
		max, parseErr := optionalPositiveInt(in.MaxUseCount)
		if parseErr != nil {
			return InviteCode{}, parseErr
		}
		exp, parseErr := optionalTime(in.ExpiresAt)
		if parseErr != nil {
			return InviteCode{}, parseErr
		}
		if max != nil && *max < usedCount {
			return InviteCode{}, apperror.New(apperror.CodeConflict, http.StatusConflict, "最大使用次数不能低于已使用次数")
		}
		err = scan(tx.QueryRowContext(ctx, `UPDATE reader_invite_codes SET code=$1,status=$2,max_use_count=$3,expires_at=$4,remark=$5,updated_at=now() WHERE id=$6 RETURNING `+`id::text,'',code,status,COALESCE(max_use_count::text,''),used_count::text,COALESCE(expires_at::text,''),remark,created_at::text`, code, status, max, exp, remark, id), &v)
	}
	if err != nil {
		return InviteCode{}, mapWriteError(err)
	}
	if err = tx.Commit(); err != nil {
		return InviteCode{}, err
	}
	return v, nil
}
func (r SQLRepository) SetStatus(ctx context.Context, id int64, status string) (InviteCode, error) {
	if status != "enabled" && status != "disabled" {
		return InviteCode{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "邀请码状态无效")
	}
	var v InviteCode
	err := scan(r.DB.QueryRowContext(ctx, `UPDATE reader_invite_codes SET status=$1,updated_at=now() WHERE id=$2 RETURNING `+`id::text,COALESCE(inviter_reader_id::text,''),code,status,COALESCE(max_use_count::text,''),used_count::text,COALESCE(expires_at::text,''),remark,created_at::text`, status, id), &v)
	if errors.Is(err, sql.ErrNoRows) {
		return InviteCode{}, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "邀请码不存在")
	}
	return v, err
}
func (r SQLRepository) Delete(ctx context.Context, id int64) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var automatic bool
	var usedCount int64
	var relationExists bool
	err = tx.QueryRowContext(ctx, `SELECT inviter_reader_id IS NOT NULL,used_count,EXISTS(SELECT 1 FROM reader_invite_relations WHERE invite_code_id=$1) FROM reader_invite_codes WHERE id=$1 FOR UPDATE`, id).Scan(&automatic, &usedCount, &relationExists)
	if errors.Is(err, sql.ErrNoRows) {
		return apperror.New(apperror.CodeNotFound, http.StatusNotFound, "邀请码不存在")
	}
	if err != nil {
		return err
	}
	if automatic {
		return apperror.New(apperror.CodeConflict, http.StatusConflict, "读者自动邀请码不能删除")
	}
	if usedCount != 0 || relationExists {
		return apperror.New(apperror.CodeConflict, http.StatusConflict, "已使用邀请码不能删除")
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM reader_invite_codes WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func mapWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return apperror.Wrap(err, apperror.CodeConflict, http.StatusConflict, "邀请码已存在")
	}
	return err
}

func optionalPositiveInt(value string) (*int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		return nil, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "最大使用次数必须是正整数字符串")
	}
	return &n, nil
}

func optionalTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "过期时间格式无效")
	}
	return &t, nil
}
