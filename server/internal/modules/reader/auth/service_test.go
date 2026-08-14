package auth

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type memoryRepository struct {
	account       ReaderAccount
	sessions      map[int64]ReaderSession
	nextSessionID int64
}

func (repo *memoryRepository) FindAccountByUsername(_ context.Context, username string) (ReaderAccount, error) {
	if repo.account.Username != username {
		return ReaderAccount{}, ErrInvalidCredentials
	}
	return repo.account, nil
}
func (repo *memoryRepository) FindAccountByID(_ context.Context, id int64) (ReaderAccount, error) {
	if repo.account.ID != id {
		return ReaderAccount{}, ErrInvalidSession
	}
	return repo.account, nil
}
func (repo *memoryRepository) UpgradePasswordAndCreateSession(_ context.Context, accountID int64, oldHash, newHash string, expiresAt time.Time) (int64, error) {
	if repo.account.ID != accountID || repo.account.PasswordHash != oldHash {
		return 0, errors.New("stale password")
	}
	repo.account.PasswordHash, repo.account.PasswordAlgorithm = newHash, PasswordAlgorithmBcrypt
	repo.account.PasswordUpgradedAt = time.Now()
	return repo.createSession(accountID, expiresAt), nil
}
func (repo *memoryRepository) CreateSession(_ context.Context, accountID int64, expiresAt time.Time) (int64, error) {
	return repo.createSession(accountID, expiresAt), nil
}
func (repo *memoryRepository) SetSessionDigest(_ context.Context, sessionID int64, digest string) error {
	s := repo.sessions[sessionID]
	s.TokenDigest = digest
	repo.sessions[sessionID] = s
	return nil
}
func (repo *memoryRepository) FindSession(_ context.Context, sessionID, readerID int64, digest string) (ReaderSession, error) {
	s, ok := repo.sessions[sessionID]
	if !ok || s.ReaderID != readerID || s.TokenDigest != digest {
		return ReaderSession{}, ErrInvalidSession
	}
	return s, nil
}
func (repo *memoryRepository) RevokeSession(_ context.Context, sessionID, readerID int64, at time.Time) error {
	s := repo.sessions[sessionID]
	if s.ReaderID != readerID {
		return ErrInvalidSession
	}
	s.RevokedAt = &at
	repo.sessions[sessionID] = s
	return nil
}
func (repo *memoryRepository) ReplacePasswordAndRevokeSessions(_ context.Context, readerID int64, oldHash, newHash string, exceptSessionID int64, at time.Time) error {
	if repo.account.ID != readerID || repo.account.PasswordHash != oldHash {
		return ErrInvalidCredentials
	}
	repo.account.PasswordHash, repo.account.PasswordAlgorithm = newHash, PasswordAlgorithmBcrypt
	for id, session := range repo.sessions {
		if id != exceptSessionID {
			session.RevokedAt = &at
			repo.sessions[id] = session
		}
	}
	return nil
}
func (repo *memoryRepository) createSession(readerID int64, expiresAt time.Time) int64 {
	if repo.sessions == nil {
		repo.sessions = map[int64]ReaderSession{}
	}
	repo.nextSessionID++
	repo.sessions[repo.nextSessionID] = ReaderSession{ID: repo.nextSessionID, ReaderID: readerID, ExpiresAt: expiresAt}
	return repo.nextSessionID
}

type fixedLimiter struct {
	allowed bool
	err     error
	keys    []string
}

func (l *fixedLimiter) Allow(_ context.Context, key string, _ int64, _ time.Duration) (bool, error) {
	l.keys = append(l.keys, key)
	return l.allowed, l.err
}

func newServiceForTest(repo Repository, limiter RateLimiter) *Service {
	return NewService(repo, limiter, TokenConfig{Secret: []byte("reader-test-signing-key-reader-test"), TTL: time.Hour, Issuer: "moonbook-reader"})
}

func TestLoginBcryptAndValidateSession(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	repo := &memoryRepository{account: ReaderAccount{ID: 9007199254740993, Username: "reader", PasswordHash: string(hash), PasswordAlgorithm: PasswordAlgorithmBcrypt, Status: AccountStatusEnabled}}
	limiter := &fixedLimiter{allowed: true}
	service := newServiceForTest(repo, limiter)
	token, err := service.Login(context.Background(), " reader ", "correct-password", "203.0.113.8")
	if err != nil {
		t.Fatal(err)
	}
	identity, err := service.ValidateToken(context.Background(), token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if identity.ReaderID != repo.account.ID || len(limiter.keys) != 2 {
		t.Fatalf("identity=%+v limiter keys=%v", identity, limiter.keys)
	}
}

func TestLoginLegacyMD5UpgradesToBcrypt(t *testing.T) {
	sum := md5.Sum([]byte("legacy-password"))
	repo := &memoryRepository{account: ReaderAccount{ID: 42, Username: "legacy", PasswordHash: hex.EncodeToString(sum[:]), PasswordAlgorithm: PasswordAlgorithmMD5, Status: AccountStatusEnabled}}
	_, err := newServiceForTest(repo, &fixedLimiter{allowed: true}).Login(context.Background(), "legacy", "legacy-password", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if repo.account.PasswordAlgorithm != PasswordAlgorithmBcrypt || bcrypt.CompareHashAndPassword([]byte(repo.account.PasswordHash), []byte("legacy-password")) != nil {
		t.Fatal("legacy password was not upgraded")
	}
}

func TestLoginRejectsUnknownDigestAndDisabledAccount(t *testing.T) {
	cases := []ReaderAccount{
		{ID: 1, Username: "reader", PasswordHash: "opaque", PasswordAlgorithm: "sha1", Status: AccountStatusEnabled},
		{ID: 1, Username: "reader", PasswordHash: "$2a$10$invalid", PasswordAlgorithm: PasswordAlgorithmBcrypt, Status: AccountStatusDisabled},
	}
	for _, account := range cases {
		repo := &memoryRepository{account: account}
		if _, err := newServiceForTest(repo, &fixedLimiter{allowed: true}).Login(context.Background(), "reader", "password", "127.0.0.1"); err == nil {
			t.Fatalf("account %+v should be rejected", account)
		}
		if repo.account.PasswordHash != account.PasswordHash {
			t.Fatal("rejected login changed password")
		}
	}
}

func TestLoginFailsClosedWhenRateLimiterUnavailableOrExceeded(t *testing.T) {
	repo := &memoryRepository{account: ReaderAccount{ID: 1, Username: "reader", Status: AccountStatusEnabled}}
	for _, limiter := range []*fixedLimiter{{allowed: false}, {allowed: false, err: errors.New("redis unavailable")}} {
		if _, err := newServiceForTest(repo, limiter).Login(context.Background(), "reader", "password", "127.0.0.1"); err == nil {
			t.Fatal("login must fail closed")
		}
	}
}

func TestLogoutAndPasswordChangeRevokeSessions(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	repo := &memoryRepository{account: ReaderAccount{ID: 7, Username: "reader", PasswordHash: string(hash), PasswordAlgorithm: PasswordAlgorithmBcrypt, Status: AccountStatusEnabled}}
	service := newServiceForTest(repo, &fixedLimiter{allowed: true})
	first, _ := service.Login(context.Background(), "reader", "old-password", "127.0.0.1")
	second, _ := service.Login(context.Background(), "reader", "old-password", "127.0.0.1")
	if err := service.ChangePassword(context.Background(), second.AccessToken, "old-password", "new-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ValidateToken(context.Background(), first.AccessToken); err == nil {
		t.Fatal("other session remained valid")
	}
	if _, err := service.ValidateToken(context.Background(), second.AccessToken); err != nil {
		t.Fatalf("current session revoked: %v", err)
	}
	if err := service.Logout(context.Background(), second.AccessToken); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ValidateToken(context.Background(), second.AccessToken); err == nil {
		t.Fatal("logged out session remained valid")
	}
}
