package invite

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
)

type transactionContextKey struct{}

type projectionWriter struct {
	projections []commercecontract.ReaderSearchProjection
	err         error
	failStage   string
}

type rewardGranter struct {
	requests                       []commercecontract.RegistrationRewardRequest
	err                            error
	failStage                      string
	inviterRewards, inviteeRewards int
}

func (granter *rewardGranter) GrantRegistrationRewards(ctx context.Context, request commercecontract.RegistrationRewardRequest) error {
	if ctx.Value(transactionContextKey{}) != true {
		return errors.New("reward did not receive transaction context")
	}
	granter.requests = append(granter.requests, request)
	granter.inviterRewards++
	if granter.failStage == "inviter_reward" {
		return errors.New("forced inviter reward failure")
	}
	granter.inviteeRewards++
	if granter.failStage == "invitee_reward" {
		return errors.New("forced invitee reward failure")
	}
	return granter.err
}

func (writer *projectionWriter) UpsertReaderSearchProjection(ctx context.Context, projection commercecontract.ReaderSearchProjection) error {
	if ctx.Value(transactionContextKey{}) != true {
		return errors.New("projection did not receive transaction context")
	}
	writer.projections = append(writer.projections, projection)
	if writer.failStage == "projection" {
		return errors.New("forced projection failure")
	}
	return writer.err
}

func (*projectionWriter) DeleteReaderSearchProjection(context.Context, int64) error {
	return errors.New("unused")
}

type registrationRepo struct {
	remaining                                         int
	accountCount, usedCount, codeCount, relationCount int
	err                                               error
	failStage                                         string
	calls                                             []string
}

func (r *registrationRepo) LockInvite(_ context.Context, code string) (InviteCode, error) {
	r.calls = append(r.calls, "invite")
	if r.err != nil {
		return InviteCode{}, r.err
	}
	if code != "VALID" || r.remaining == 0 {
		return InviteCode{}, ErrInviteUsed
	}
	maximum := 1
	return InviteCode{ID: 10, InviterReaderID: 99, Code: code, Status: "enabled", MaxUseCount: &maximum}, nil
}

func (r *registrationRepo) CreateAccount(_ context.Context, req Registration, inviteCodeID int64) (auth.ReaderAccount, error) {
	r.calls = append(r.calls, "account")
	r.accountCount++
	if r.failStage == "account" {
		return auth.ReaderAccount{}, errors.New("forced account failure")
	}
	return auth.ReaderAccount{ID: int64(r.accountCount + 100), Username: req.Username, Nickname: req.Nickname, Status: auth.AccountStatusEnabled}, nil
}

func (r *registrationRepo) IncrementInviteUsage(context.Context, int64) error {
	r.calls = append(r.calls, "invite_usage")
	r.remaining--
	r.usedCount++
	if r.failStage == "invite_usage" {
		return errors.New("forced invite usage failure")
	}
	return nil
}

func (r *registrationRepo) CreateAutomaticInviteCode(context.Context, int64) error {
	r.calls = append(r.calls, "automatic_code")
	r.codeCount++
	if r.failStage == "automatic_code" {
		return errors.New("forced automatic code failure")
	}
	return nil
}

func (r *registrationRepo) CreateInviteRelation(context.Context, int64, int64, int64) (int64, error) {
	r.calls = append(r.calls, "relation")
	r.relationCount++
	if r.failStage == "relation" {
		return 0, errors.New("forced relation failure")
	}
	return int64(r.relationCount + 200), nil
}

type rollbackTransactor struct {
	mu          sync.Mutex
	repo        *registrationRepo
	projections *projectionWriter
	rewards     *rewardGranter
}

func (tx *rollbackTransactor) Within(ctx context.Context, fn func(context.Context) error) error {
	tx.mu.Lock()
	defer tx.mu.Unlock()
	repoSnapshot := *tx.repo
	projectionCount := len(tx.projections.projections)
	rewardCount := len(tx.rewards.requests)
	inviterRewards, inviteeRewards := tx.rewards.inviterRewards, tx.rewards.inviteeRewards
	err := fn(context.WithValue(ctx, transactionContextKey{}, true))
	if err != nil {
		*tx.repo = repoSnapshot
		tx.projections.projections = tx.projections.projections[:projectionCount]
		tx.rewards.requests = tx.rewards.requests[:rewardCount]
		tx.rewards.inviterRewards = inviterRewards
		tx.rewards.inviteeRewards = inviteeRewards
	}
	return err
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

func testService(repo Repository) (*Service, *sessionRepo, *projectionWriter, *rewardGranter) {
	sessions := &sessionRepo{}
	authService := auth.NewService(sessions, nil, auth.TokenConfig{Secret: []byte("reader-invite-test-signing-key"), TTL: time.Hour})
	projections := &projectionWriter{}
	rewards := &rewardGranter{}
	registration, ok := repo.(*registrationRepo)
	if !ok {
		panic("testService requires registrationRepo")
	}
	transactor := &rollbackTransactor{repo: registration, projections: projections, rewards: rewards}
	return NewService(repo, authService, transactor, projections, rewards), sessions, projections, rewards
}

func TestRegisterRequiresInviteCode(t *testing.T) {
	service, _, _, _ := testService(&registrationRepo{remaining: 1})
	_, _, err := service.Register(context.Background(), auth.RegisterRequest{Username: "reader", Password: "password"})
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("error = %v", err)
	}
}

func TestRegisterCreatesAccountCodeAndRelation(t *testing.T) {
	repo := &registrationRepo{remaining: 1}
	service, _, projections, rewards := testService(repo)
	account, token, err := service.Register(context.Background(), auth.RegisterRequest{Username: " reader ", Password: "password", Nickname: "Moon", InviteCode: " VALID "})
	if err != nil {
		t.Fatal(err)
	}
	if account.Username != "reader" || token.AccessToken == "" {
		t.Fatalf("account=%+v token=%+v", account, token)
	}
	if repo.accountCount != 1 || repo.usedCount != 1 || repo.codeCount != 1 || repo.relationCount != 1 {
		t.Fatalf("facts=%+v", repo)
	}
	if len(projections.projections) != 1 || projections.projections[0] != (commercecontract.ReaderSearchProjection{ReaderID: account.ID, Username: "reader", Nickname: "Moon", Status: auth.AccountStatusEnabled}) {
		t.Fatalf("projections=%+v", projections.projections)
	}
	if len(rewards.requests) != 1 || rewards.requests[0].InviterID != 99 || rewards.requests[0].InviteeID != account.ID {
		t.Fatalf("rewards=%+v", rewards.requests)
	}
}

func TestRegisterDoesNotCreateSessionWhenProjectionFails(t *testing.T) {
	repo := &registrationRepo{remaining: 1}
	service, sessions, projections, rewards := testService(repo)
	projections.err = errors.New("projection unavailable")

	_, token, err := service.Register(context.Background(), auth.RegisterRequest{Username: "reader", Password: "password", InviteCode: "VALID"})
	if err == nil || token.AccessToken != "" {
		t.Fatalf("token=%+v err=%v", token, err)
	}
	if sessions.next.Load() != 0 {
		t.Fatalf("sessions created before transaction commit: %d", sessions.next.Load())
	}
	if len(rewards.requests) != 0 {
		t.Fatalf("reward ran after projection failure: %+v", rewards.requests)
	}
	if repo.accountCount != 0 || repo.usedCount != 0 || repo.codeCount != 0 || repo.relationCount != 0 || repo.remaining != 1 {
		t.Fatalf("Reader facts were not rolled back: %+v", repo)
	}
}

func TestRegistrationFailureMatrixRollsBackEveryFact(t *testing.T) {
	for _, stage := range []string{"account", "invite_usage", "automatic_code", "relation", "projection", "inviter_reward", "invitee_reward"} {
		t.Run(stage, func(t *testing.T) {
			repo := &registrationRepo{remaining: 1, failStage: stage}
			service, sessions, projections, rewards := testService(repo)
			projections.failStage = stage
			rewards.failStage = stage
			if _, _, err := service.Register(context.Background(), auth.RegisterRequest{Username: "reader", Password: "password", InviteCode: "VALID"}); err == nil {
				t.Fatal("expected registration failure")
			}
			if repo.remaining != 1 || repo.accountCount != 0 || repo.usedCount != 0 || repo.codeCount != 0 || repo.relationCount != 0 {
				t.Fatalf("Reader facts remained after %s: %+v", stage, repo)
			}
			if len(projections.projections) != 0 || len(rewards.requests) != 0 || rewards.inviterRewards != 0 || rewards.inviteeRewards != 0 || sessions.next.Load() != 0 {
				t.Fatalf("cross-domain facts remained after %s: projections=%v rewards=%v/%d/%d sessions=%d", stage, projections.projections, rewards.requests, rewards.inviterRewards, rewards.inviteeRewards, sessions.next.Load())
			}
		})
	}
}

func TestRegisterDoesNotChangeProjectionWhenReaderWriteFails(t *testing.T) {
	want := errors.New("reader write failed")
	service, sessions, projections, rewards := testService(&registrationRepo{remaining: 1, err: want})

	if _, _, err := service.Register(context.Background(), auth.RegisterRequest{Username: "reader", Password: "password", InviteCode: "VALID"}); !errors.Is(err, want) {
		t.Fatalf("error=%v", err)
	}
	if len(projections.projections) != 0 || len(rewards.requests) != 0 || sessions.next.Load() != 0 {
		t.Fatalf("projections=%+v rewards=%+v sessions=%d", projections.projections, rewards.requests, sessions.next.Load())
	}
}

func TestConcurrentRegistrationConsumesLimitedInviteOnce(t *testing.T) {
	repo := &registrationRepo{remaining: 1}
	service, _, _, _ := testService(repo)
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
