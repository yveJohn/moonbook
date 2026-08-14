package invite

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
)

type registrationRepo struct {
	mu                                                sync.Mutex
	remaining                                         int
	accountCount, usedCount, codeCount, relationCount int
}

func (r *registrationRepo) RegisterWithInvite(_ context.Context, req Registration) (auth.ReaderAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if req.InviteCode != "VALID" || r.remaining == 0 {
		return auth.ReaderAccount{}, ErrInviteUsed
	}
	r.remaining--
	r.accountCount++
	r.usedCount++
	r.codeCount++
	r.relationCount++
	return auth.ReaderAccount{ID: int64(r.accountCount + 100), Username: req.Username, Nickname: req.Nickname, Status: auth.AccountStatusEnabled}, nil
}

type sessionRepo struct{ next atomic.Int64 }

func (*sessionRepo) FindAccountByUsername(context.Context, string) (auth.ReaderAccount, error) {
	return auth.ReaderAccount{}, errors.New("unused")
}
func (*sessionRepo) FindAccountByID(context.Context, int64) (auth.ReaderAccount, error) {
	return auth.ReaderAccount{}, errors.New("unused")
}
func (r *sessionRepo) CreateSession(context.Context, int64, time.Time) (int64, error) {
	return r.next.Add(1), nil
}
func (*sessionRepo) UpgradePasswordAndCreateSession(context.Context, int64, string, string, time.Time) (int64, error) {
	return 0, errors.New("unused")
}
func (*sessionRepo) SetSessionDigest(context.Context, int64, string) error { return nil }
func (*sessionRepo) FindSession(context.Context, int64, int64, string) (auth.ReaderSession, error) {
	return auth.ReaderSession{}, errors.New("unused")
}
func (*sessionRepo) RevokeSession(context.Context, int64, int64, time.Time) error {
	return errors.New("unused")
}
func (*sessionRepo) ReplacePasswordAndRevokeSessions(context.Context, int64, string, string, int64, time.Time) error {
	return errors.New("unused")
}

func testService(repo Repository) *Service {
	authService := auth.NewService(&sessionRepo{}, nil, auth.TokenConfig{Secret: []byte("reader-invite-test-signing-key"), TTL: time.Hour})
	return NewService(repo, authService)
}

func TestRegisterRequiresInviteCode(t *testing.T) {
	_, _, err := testService(&registrationRepo{remaining: 1}).Register(context.Background(), auth.RegisterRequest{Username: "reader", Password: "password"})
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("error = %v", err)
	}
}

func TestRegisterCreatesAccountCodeAndRelation(t *testing.T) {
	repo := &registrationRepo{remaining: 1}
	account, token, err := testService(repo).Register(context.Background(), auth.RegisterRequest{Username: " reader ", Password: "password", Nickname: "Moon", InviteCode: " VALID "})
	if err != nil {
		t.Fatal(err)
	}
	if account.Username != "reader" || token.AccessToken == "" {
		t.Fatalf("account=%+v token=%+v", account, token)
	}
	if repo.accountCount != 1 || repo.usedCount != 1 || repo.codeCount != 1 || repo.relationCount != 1 {
		t.Fatalf("facts=%+v", repo)
	}
}

func TestConcurrentRegistrationConsumesLimitedInviteOnce(t *testing.T) {
	repo := &registrationRepo{remaining: 1}
	service := testService(repo)
	var wg sync.WaitGroup
	var successes atomic.Int64
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := service.Register(context.Background(), auth.RegisterRequest{Username: "reader", Password: "password", InviteCode: "VALID"}); err == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("successful registrations = %d", successes.Load())
	}
	if repo.accountCount != 1 || repo.usedCount != 1 || repo.codeCount != 1 || repo.relationCount != 1 {
		t.Fatalf("partial facts remained: %+v", repo)
	}
}
