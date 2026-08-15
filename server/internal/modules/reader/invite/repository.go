package invite

import (
	"context"
	"database/sql"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
	"golang.org/x/crypto/bcrypt"
)

func (r SQLRepository) transactionExecutor(ctx context.Context) (transaction.DBTX, error) {
	executor := transaction.Executor(ctx, r.DB)
	if executor == r.DB {
		return nil, transaction.ErrNoTransaction
	}
	return executor, nil
}

func (r SQLRepository) LockInvite(ctx context.Context, code string) (InviteCode, error) {
	executor, err := r.transactionExecutor(ctx)
	if err != nil {
		return InviteCode{}, err
	}
	// ID allocation locks precede every Reader row lock in the registration flow.
	if _, err = executor.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('reader-registration-id',0))`); err != nil {
		return InviteCode{}, err
	}
	if _, err = executor.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('reader-invite-id',0))`); err != nil {
		return InviteCode{}, err
	}
	var invite InviteCode
	var max sql.NullInt64
	var expires sql.NullTime
	err = executor.QueryRowContext(ctx, `SELECT id,COALESCE(inviter_reader_id,0),code,status,max_use_count,used_count,expires_at FROM reader_invite_codes WHERE code=$1 FOR UPDATE`, code).
		Scan(&invite.ID, &invite.InviterReaderID, &invite.Code, &invite.Status, &max, &invite.UsedCount, &expires)
	if err != nil {
		return InviteCode{}, ErrInviteInvalid
	}
	if max.Valid {
		v := int(max.Int64)
		invite.MaxUseCount = &v
	}
	if expires.Valid {
		invite.ExpiresAt = &expires.Time
	}
	if invite.Status != "enabled" || (invite.ExpiresAt != nil && !invite.ExpiresAt.After(time.Now())) {
		return InviteCode{}, ErrInviteInvalid
	}
	if invite.MaxUseCount != nil && invite.UsedCount >= *invite.MaxUseCount {
		return InviteCode{}, ErrInviteUsed
	}
	return invite, nil
}

func (r SQLRepository) CreateAccount(ctx context.Context, req Registration, inviteCodeID int64) (auth.ReaderAccount, error) {
	executor, err := r.transactionExecutor(ctx)
	if err != nil {
		return auth.ReaderAccount{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return auth.ReaderAccount{}, err
	}
	var id int64
	if err = executor.QueryRowContext(ctx, `SELECT COALESCE(MAX(id),0)+1 FROM reader_accounts`).Scan(&id); err != nil {
		return auth.ReaderAccount{}, err
	}
	var account auth.ReaderAccount
	err = executor.QueryRowContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash,password_algorithm,status,invite_code_id) VALUES($1,$2,$3,$4,'bcrypt','enabled',$5) RETURNING id,username,nickname,password_hash,password_algorithm,status,COALESCE(password_upgraded_at,'epoch')`, id, req.Username, req.Nickname, string(hash), inviteCodeID).
		Scan(&account.ID, &account.Username, &account.Nickname, &account.PasswordHash, &account.PasswordAlgorithm, &account.Status, &account.PasswordUpgradedAt)
	if err != nil {
		return auth.ReaderAccount{}, err
	}
	return account, nil
}

func (r SQLRepository) IncrementInviteUsage(ctx context.Context, inviteCodeID int64) error {
	executor, err := r.transactionExecutor(ctx)
	if err != nil {
		return err
	}
	result, err := executor.ExecContext(ctx, `UPDATE reader_invite_codes SET used_count=used_count+1,updated_at=now() WHERE id=$1`, inviteCodeID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return ErrInviteInvalid
	}
	return nil
}

func (r SQLRepository) CreateAutomaticInviteCode(ctx context.Context, readerID int64) error {
	executor, err := r.transactionExecutor(ctx)
	if err != nil {
		return err
	}
	code, err := generatedCode()
	if err != nil {
		return err
	}
	if _, err = executor.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status,max_use_count,used_count) VALUES((SELECT COALESCE(MAX(id),0)+1 FROM reader_invite_codes),$1,$2,'enabled',NULL,0) ON CONFLICT (inviter_reader_id) WHERE inviter_reader_id IS NOT NULL DO NOTHING`, code, readerID); err != nil {
		return err
	}
	return nil
}

func (r SQLRepository) CreateInviteRelation(ctx context.Context, inviterID, inviteeID, inviteCodeID int64) (int64, error) {
	executor, err := r.transactionExecutor(ctx)
	if err != nil {
		return 0, err
	}
	var relationID int64
	err = executor.QueryRowContext(ctx, `INSERT INTO reader_invite_relations(inviter_reader_id,invitee_reader_id,invite_code_id) VALUES($1,$2,$3) ON CONFLICT (invitee_reader_id) DO NOTHING RETURNING id`, inviterID, inviteeID, inviteCodeID).Scan(&relationID)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return relationID, err
}
