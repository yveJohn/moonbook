package invite

import (
	"context"
	"database/sql"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"golang.org/x/crypto/bcrypt"
)

func (r SQLRepository) RegisterWithInvite(ctx context.Context, req Registration) (auth.ReaderAccount, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return auth.ReaderAccount{}, err
	}
	defer tx.Rollback()
	var invite InviteCode
	var max sql.NullInt64
	var expires sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT id,COALESCE(inviter_reader_id,0),code,status,max_use_count,used_count,expires_at FROM reader_invite_codes WHERE code=$1 FOR UPDATE`, req.InviteCode).
		Scan(&invite.ID, &invite.InviterReaderID, &invite.Code, &invite.Status, &max, &invite.UsedCount, &expires)
	if err != nil {
		return auth.ReaderAccount{}, ErrInviteInvalid
	}
	if max.Valid {
		v := int(max.Int64)
		invite.MaxUseCount = &v
	}
	if expires.Valid {
		invite.ExpiresAt = &expires.Time
	}
	if invite.Status != "enabled" || (invite.ExpiresAt != nil && !invite.ExpiresAt.After(time.Now())) {
		return auth.ReaderAccount{}, ErrInviteInvalid
	}
	if invite.MaxUseCount != nil && invite.UsedCount >= *invite.MaxUseCount {
		return auth.ReaderAccount{}, ErrInviteUsed
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return auth.ReaderAccount{}, err
	}
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('reader-registration-id', 0))`); err != nil {
		return auth.ReaderAccount{}, err
	}
	var id int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(id),0)+1 FROM reader_accounts`).Scan(&id); err != nil {
		return auth.ReaderAccount{}, err
	}
	var account auth.ReaderAccount
	err = tx.QueryRowContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash,password_algorithm,status,invite_code_id) VALUES($1,$2,$3,$4,'bcrypt','enabled',$5) RETURNING id,username,nickname,password_hash,password_algorithm,status,COALESCE(password_upgraded_at,'epoch')`, id, req.Username, req.Nickname, string(hash), invite.ID).
		Scan(&account.ID, &account.Username, &account.Nickname, &account.PasswordHash, &account.PasswordAlgorithm, &account.Status, &account.PasswordUpgradedAt)
	if err != nil {
		return auth.ReaderAccount{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE reader_invite_codes SET used_count=used_count+1,updated_at=now() WHERE id=$1`, invite.ID); err != nil {
		return auth.ReaderAccount{}, err
	}
	code, err := generatedCode()
	if err != nil {
		return auth.ReaderAccount{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status,max_use_count,used_count) VALUES((SELECT COALESCE(MAX(id),0)+1 FROM reader_invite_codes),$1,$2,'enabled',NULL,0) ON CONFLICT (inviter_reader_id) WHERE inviter_reader_id IS NOT NULL DO NOTHING`, code, account.ID); err != nil {
		return auth.ReaderAccount{}, err
	}
	if invite.InviterReaderID != 0 {
		if _, err = tx.ExecContext(ctx, `INSERT INTO reader_invite_relations(inviter_reader_id,invitee_reader_id,invite_code_id) VALUES($1,$2,$3) ON CONFLICT (invitee_reader_id) DO NOTHING`, invite.InviterReaderID, account.ID, invite.ID); err != nil {
			return auth.ReaderAccount{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return auth.ReaderAccount{}, err
	}
	return account, nil
}
