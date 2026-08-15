package invite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"

	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

var (
	ErrInviteInvalid           = apperror.New(apperror.CodeInvalidArgument, 200, "邀请码无效或已失效")
	ErrInviteUsed              = apperror.New(apperror.CodeConflict, 200, "邀请码已达到使用上限")
	ErrRegistrationUnavailable = apperror.New(apperror.CodeUnavailable, 503, "认证服务暂不可用")
)

type Service struct {
	repo   Repository
	auth   *auth.Service
	tx     Transactor
	sync   commercecontract.ReaderSearchProjectionWriter
	reward commercecontract.RegistrationRewardGranter
}

func NewService(repo Repository, authService *auth.Service, transactor Transactor, projectionSync commercecontract.ReaderSearchProjectionWriter, rewardGranter commercecontract.RegistrationRewardGranter) *Service {
	return &Service{repo: repo, auth: authService, tx: transactor, sync: projectionSync, reward: rewardGranter}
}

// Register creates Reader and Commerce facts in one context-bound transaction.
// Token/session creation starts only after every business fact commits.
func (s *Service) Register(ctx context.Context, req auth.RegisterRequest) (auth.ReaderAccount, auth.AccessToken, error) {
	if strings.TrimSpace(req.Username) == "" || req.Password == "" || strings.TrimSpace(req.InviteCode) == "" {
		return auth.ReaderAccount{}, auth.AccessToken{}, auth.ErrInvalidCredentials
	}
	if s.repo == nil || s.auth == nil || s.tx == nil || s.sync == nil || s.reward == nil {
		return auth.ReaderAccount{}, auth.AccessToken{}, ErrRegistrationUnavailable
	}
	var account auth.ReaderAccount
	err := s.tx.Within(ctx, func(txCtx context.Context) error {
		registration := Registration{Username: strings.TrimSpace(req.Username), Password: req.Password, Nickname: strings.TrimSpace(req.Nickname), InviteCode: strings.TrimSpace(req.InviteCode)}
		invite, err := s.repo.LockInvite(txCtx, registration.InviteCode)
		if err != nil {
			return err
		}
		account, err = s.repo.CreateAccount(txCtx, registration, invite.ID)
		if err != nil {
			return err
		}
		if err := s.repo.IncrementInviteUsage(txCtx, invite.ID); err != nil {
			return err
		}
		if err := s.repo.CreateAutomaticInviteCode(txCtx, account.ID); err != nil {
			return err
		}
		var relationID int64
		if invite.InviterReaderID != 0 {
			relationID, err = s.repo.CreateInviteRelation(txCtx, invite.InviterReaderID, account.ID, invite.ID)
			if err != nil {
				return err
			}
		}
		if err := s.sync.UpsertReaderSearchProjection(txCtx, commercecontract.ReaderSearchProjection{
			ReaderID: account.ID,
			Username: account.Username,
			Nickname: account.Nickname,
			Status:   account.Status,
		}); err != nil {
			return err
		}
		if relationID != 0 {
			if err := s.reward.GrantRegistrationRewards(txCtx, commercecontract.RegistrationRewardRequest{
				RelationID: relationID,
				InviterID:  invite.InviterReaderID,
				InviteeID:  account.ID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
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
