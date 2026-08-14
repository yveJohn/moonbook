package auth

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

const (
	PasswordAlgorithmBcrypt = "bcrypt"
	PasswordAlgorithmMD5    = "md5"
	AccountStatusEnabled    = "enabled"
	AccountStatusDisabled   = "disabled"
	AccountStatusDeleted    = "deleted"
)

var (
	ErrInvalidCredentials = apperror.New(apperror.CodeUnauthenticated, 200, "用户名或密码错误")
	ErrInvalidSession     = apperror.New(apperror.CodeUnauthenticated, 200, "登录状态已失效")
	ErrRateLimited        = apperror.New(apperror.CodeRateLimited, 200, "请求过于频繁，请稍后再试")
	ErrAuthUnavailable    = apperror.New(apperror.CodeUnavailable, 503, "认证服务暂不可用")
)

type ReaderAccount struct {
	ID                                                          int64
	Username, Nickname, PasswordHash, PasswordAlgorithm, Status string
	PasswordUpgradedAt                                          time.Time
}
type ReaderSession struct {
	ID, ReaderID int64
	TokenDigest  string
	ExpiresAt    time.Time
	RevokedAt    *time.Time
}
type Identity struct{ ReaderID, SessionID int64 }
type AccessToken struct {
	AccessToken string
	ExpiresAt   time.Time
}
type TokenConfig struct {
	Secret []byte
	TTL    time.Duration
	Issuer string
}

type Repository interface {
	FindAccountByUsername(context.Context, string) (ReaderAccount, error)
	FindAccountByID(context.Context, int64) (ReaderAccount, error)
	CreateSession(context.Context, int64, time.Time) (int64, error)
	UpgradePasswordAndCreateSession(context.Context, int64, string, string, time.Time) (int64, error)
	SetSessionDigest(context.Context, int64, string) error
	FindSession(context.Context, int64, int64, string) (ReaderSession, error)
	RevokeSession(context.Context, int64, int64, time.Time) error
	ReplacePasswordAndRevokeSessions(context.Context, int64, string, string, int64, time.Time) error
}
type RateLimiter interface {
	Allow(context.Context, string, int64, time.Duration) (bool, error)
}
type Service struct {
	repo    Repository
	limiter RateLimiter
	tokens  TokenConfig
	limit   int64
	window  time.Duration
}

func NewService(repo Repository, limiter RateLimiter, config TokenConfig) *Service {
	if config.TTL <= 0 {
		config.TTL = 7 * 24 * time.Hour
	}
	if config.Issuer == "" {
		config.Issuer = "moonbook-reader"
	}
	return &Service{repo: repo, limiter: limiter, tokens: config, limit: 10, window: time.Minute}
}

func (s *Service) Login(ctx context.Context, username, password, ip string) (AccessToken, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return AccessToken{}, ErrInvalidCredentials
	}
	if err := s.checkLimit(ctx, ip, username); err != nil {
		return AccessToken{}, err
	}
	account, err := s.repo.FindAccountByUsername(ctx, username)
	if err != nil {
		return AccessToken{}, ErrInvalidCredentials
	}
	if account.Status != AccountStatusEnabled {
		return AccessToken{}, ErrInvalidCredentials
	}
	upgrade := false
	switch account.PasswordAlgorithm {
	case PasswordAlgorithmBcrypt:
		if bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(password)) != nil {
			return AccessToken{}, ErrInvalidCredentials
		}
	case PasswordAlgorithmMD5:
		sum := md5.Sum([]byte(password))
		if !strings.EqualFold(hex.EncodeToString(sum[:]), strings.TrimSpace(account.PasswordHash)) {
			return AccessToken{}, ErrInvalidCredentials
		}
		upgrade = true
	default:
		return AccessToken{}, ErrInvalidCredentials
	}
	expires := time.Now().Add(s.tokens.TTL)
	var sessionID int64
	if upgrade {
		hash, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if e != nil {
			return AccessToken{}, apperror.Wrap(e, apperror.CodeInternal, 500, "登录失败")
		}
		sessionID, err = s.repo.UpgradePasswordAndCreateSession(ctx, account.ID, account.PasswordHash, string(hash), expires)
	} else {
		sessionID, err = s.repo.CreateSession(ctx, account.ID, expires)
	}
	if err != nil {
		return AccessToken{}, apperror.Wrap(err, apperror.CodeInternal, 500, "登录失败")
	}
	token, err := s.issue(account.ID, sessionID, expires)
	if err != nil {
		return AccessToken{}, err
	}
	if err := s.repo.SetSessionDigest(ctx, sessionID, digest(token)); err != nil {
		return AccessToken{}, apperror.Wrap(err, apperror.CodeInternal, 500, "登录失败")
	}
	return AccessToken{AccessToken: token, ExpiresAt: expires}, nil
}

// CreateToken creates a Reader session for an account that has just been
// committed by another Reader transaction (for example invitation signup).
// It intentionally does not perform password checks or rate-limit accounting;
// the caller must have completed those checks before invoking it.
func (s *Service) CreateToken(ctx context.Context, readerID int64) (AccessToken, error) {
	expires := time.Now().Add(s.tokens.TTL)
	sessionID, err := s.repo.CreateSession(ctx, readerID, expires)
	if err != nil {
		return AccessToken{}, apperror.Wrap(err, apperror.CodeInternal, 500, "登录失败")
	}
	token, err := s.issue(readerID, sessionID, expires)
	if err != nil {
		return AccessToken{}, err
	}
	if err = s.repo.SetSessionDigest(ctx, sessionID, digest(token)); err != nil {
		return AccessToken{}, apperror.Wrap(err, apperror.CodeInternal, 500, "登录失败")
	}
	return AccessToken{AccessToken: token, ExpiresAt: expires}, nil
}

func (s *Service) checkLimit(ctx context.Context, ip, username string) error {
	if s.limiter == nil {
		return ErrAuthUnavailable
	}
	for _, key := range []string{"ip:" + strings.TrimSpace(ip), "account:" + strings.ToLower(strings.TrimSpace(username))} {
		ok, err := s.limiter.Allow(ctx, key, s.limit, s.window)
		if err != nil {
			return ErrAuthUnavailable
		}
		if !ok {
			return ErrRateLimited
		}
	}
	return nil
}
func (s *Service) issue(readerID, sessionID int64, expires time.Time) (string, error) {
	claims := jwt.MapClaims{"reader_id": fmt.Sprintf("%d", readerID), "session_id": fmt.Sprintf("%d", sessionID), "iss": s.tokens.Issuer, "exp": expires.Unix()}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.tokens.Secret)
}
func (s *Service) parse(raw string) (Identity, error) {
	token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return s.tokens.Secret, nil
	}, jwt.WithIssuer(s.tokens.Issuer))
	if err != nil || !token.Valid {
		return Identity{}, ErrInvalidSession
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return Identity{}, ErrInvalidSession
	}
	var id, sid int64
	readerClaim, ok := claims["reader_id"].(string)
	sessionClaim, ok2 := claims["session_id"].(string)
	if !ok || !ok2 {
		return Identity{}, ErrInvalidSession
	}
	if _, e := fmt.Sscan(readerClaim, &id); e != nil {
		return Identity{}, ErrInvalidSession
	}
	if _, e := fmt.Sscan(sessionClaim, &sid); e != nil {
		return Identity{}, ErrInvalidSession
	}
	return Identity{ReaderID: id, SessionID: sid}, nil
}
func (s *Service) ValidateToken(ctx context.Context, raw string) (Identity, error) {
	identity, err := s.parse(raw)
	if err != nil {
		return Identity{}, err
	}
	account, err := s.repo.FindAccountByID(ctx, identity.ReaderID)
	if err != nil || account.Status != AccountStatusEnabled {
		return Identity{}, ErrInvalidSession
	}
	session, err := s.repo.FindSession(ctx, identity.SessionID, identity.ReaderID, digest(raw))
	if err != nil || session.RevokedAt != nil || !session.ExpiresAt.After(time.Now()) {
		return Identity{}, ErrInvalidSession
	}
	return identity, nil
}
func (s *Service) Logout(ctx context.Context, raw string) error {
	identity, err := s.ValidateToken(ctx, raw)
	if err != nil {
		return err
	}
	return s.repo.RevokeSession(ctx, identity.SessionID, identity.ReaderID, time.Now())
}
func (s *Service) ChangePassword(ctx context.Context, raw, oldPassword, newPassword string) error {
	identity, err := s.ValidateToken(ctx, raw)
	if err != nil {
		return err
	}
	account, err := s.repo.FindAccountByID(ctx, identity.ReaderID)
	if err != nil || account.Status != AccountStatusEnabled {
		return ErrInvalidCredentials
	}
	if account.PasswordAlgorithm == PasswordAlgorithmBcrypt {
		if bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(oldPassword)) != nil {
			return ErrInvalidCredentials
		}
	} else if account.PasswordAlgorithm == PasswordAlgorithmMD5 {
		sum := md5.Sum([]byte(oldPassword))
		if !strings.EqualFold(hex.EncodeToString(sum[:]), account.PasswordHash) {
			return ErrInvalidCredentials
		}
	} else {
		return ErrInvalidCredentials
	}
	if strings.TrimSpace(newPassword) == "" {
		return ErrInvalidCredentials
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if e != nil {
		return apperror.Wrap(e, apperror.CodeInternal, 500, "修改密码失败")
	}
	if e = s.repo.ReplacePasswordAndRevokeSessions(ctx, identity.ReaderID, account.PasswordHash, string(hash), identity.SessionID, time.Now()); e != nil {
		return apperror.Wrap(e, apperror.CodeInternal, 500, "修改密码失败")
	}
	return nil
}
func digest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type RedisRateLimiter struct {
	Client redis.UniversalClient
	Prefix string
}

func (r RedisRateLimiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	if r.Client == nil {
		return false, ErrAuthUnavailable
	}
	k := r.Prefix + key
	n, err := r.Client.Incr(ctx, k).Result()
	if err != nil {
		return false, err
	}
	if n == 1 {
		if err = r.Client.Expire(ctx, k, window).Err(); err != nil {
			return false, err
		}
	}
	return n <= limit, nil
}
