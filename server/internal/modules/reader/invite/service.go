package invite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

var (
	ErrInviteInvalid           = apperror.New(apperror.CodeInvalidArgument, 200, "邀请码无效或已失效")
	ErrInviteUsed              = apperror.New(apperror.CodeConflict, 200, "邀请码已达到使用上限")
	ErrRegistrationUnavailable = apperror.New(apperror.CodeUnavailable, 503, "认证服务暂不可用")
)

type Service struct {
	repo Repository
	auth *auth.Service
}

func NewService(repo Repository, authService *auth.Service) *Service {
	return &Service{repo: repo, auth: authService}
}

// Register consumes the invitation and creates the account and relation in
// one repository transaction. Token/session creation happens only after that
// transaction commits and does not create any wallet or reward fact.
func (s *Service) Register(ctx context.Context, req auth.RegisterRequest) (auth.ReaderAccount, auth.AccessToken, error) {
	if strings.TrimSpace(req.Username) == "" || req.Password == "" || strings.TrimSpace(req.InviteCode) == "" {
		return auth.ReaderAccount{}, auth.AccessToken{}, auth.ErrInvalidCredentials
	}
	if s.repo == nil || s.auth == nil {
		return auth.ReaderAccount{}, auth.AccessToken{}, ErrRegistrationUnavailable
	}
	account, err := s.repo.RegisterWithInvite(ctx, Registration{Username: strings.TrimSpace(req.Username), Password: req.Password, Nickname: strings.TrimSpace(req.Nickname), InviteCode: strings.TrimSpace(req.InviteCode)})
	if err != nil {
		return auth.ReaderAccount{}, auth.AccessToken{}, err
	}
	token, err := s.auth.CreateToken(ctx, account.ID)
	if err != nil {
		return auth.ReaderAccount{}, auth.AccessToken{}, err
	}
	return account, token, nil
}

func generatedCode() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "MB-" + strings.ToUpper(hex.EncodeToString(b)), nil
}

// GenerateCodeForReader creates the reader's one-per-account invite code in an
// existing transaction. The caller must hold the inviter row lock.
func GenerateCodeForReader(ctx context.Context, tx *sql.Tx, readerID int64) (string, error) {
	code, err := generatedCode()
	if err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('reader-invite-id',0))`); err != nil {
		return "", err
	}
	var out string
	err = tx.QueryRowContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status,max_use_count,used_count) VALUES((SELECT COALESCE(MAX(id),0)+1 FROM reader_invite_codes),$1,$2,'enabled',NULL,0) ON CONFLICT (inviter_reader_id) DO UPDATE SET updated_at=reader_invite_codes.updated_at RETURNING code`, code, readerID).Scan(&out)
	return out, err
}

func isRetryableInviteError(err error) bool { return errors.Is(err, ErrInviteInvalid) }
