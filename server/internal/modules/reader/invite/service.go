package invite

import (
	"context"
	"crypto/rand"
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

func isRetryableInviteError(err error) bool { return errors.Is(err, ErrInviteInvalid) }
