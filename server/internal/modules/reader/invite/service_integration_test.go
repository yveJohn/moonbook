//go:build integration

package invite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	commerceprovider "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/provider"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
	"golang.org/x/crypto/bcrypt"
)

func TestReaderRegistrationConsumesInviteAndCreatesRelation(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	inviter, inviteCodeID := base, base+1
	var invitee int64
	code := fmt.Sprintf("MB-IT-%d", base)
	inviterName := integrationtest.Prefix() + "-inviter"
	inviteeName := integrationtest.Prefix() + "-invitee"
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("inviter-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash,status) VALUES($1,$2,'邀请人',$3,'enabled')`, inviter, inviterName, string(passwordHash)); err != nil {
		t.Fatal(err)
	}
	restoreRewardConfig := setInviteRewardConfig(t, db, ctx, true, 13, 7)
	t.Cleanup(restoreRewardConfig)
	if _, err = db.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status,max_use_count) VALUES($1,$2,$3,'enabled',1)`, inviteCodeID, code, inviter); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_sessions WHERE reader_id IN ($1,$2)`, inviter, invitee)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_invite_reward_records WHERE inviter_reader_id IN ($1,$2) OR invitee_reader_id IN ($1,$2)`, inviter, invitee)
		_, _ = db.ExecContext(cleanup, `ALTER TABLE reader_wallet_ledgers DISABLE TRIGGER reader_wallet_ledgers_immutable_update`)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_wallet_ledgers WHERE reader_id IN ($1,$2)`, inviter, invitee)
		_, _ = db.ExecContext(cleanup, `ALTER TABLE reader_wallet_ledgers ENABLE TRIGGER reader_wallet_ledgers_immutable_update`)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_wallets WHERE reader_id IN ($1,$2)`, inviter, invitee)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_invite_relations WHERE inviter_reader_id IN ($1,$2) OR invitee_reader_id IN ($1,$2)`, inviter, invitee)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_invite_codes WHERE id=$3 OR inviter_reader_id IN ($1,$2)`, inviter, invitee, inviteCodeID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM commerce_reader_search_projection WHERE reader_id IN ($1,$2)`, inviter, invitee)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_accounts WHERE id IN ($1,$2)`, inviter, invitee)
	})
	authService := auth.NewService(auth.SQLRepository{DB: db}, nil, auth.TokenConfig{Secret: []byte("integration-registration-secret-32"), TTL: time.Hour, Issuer: "moonbook-reader-registration"})
	registration := NewService(SQLRepository{DB: db}, authService, transaction.New(db), commerceprovider.NewReaderSearch(db), commerceprovider.NewRegistrationReward(db))
	account, token, err := registration.Register(ctx, auth.RegisterRequest{Username: inviteeName, Password: "invitee-password", Nickname: "被邀请人", InviteCode: code})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	invitee = account.ID
	if account.PasswordAlgorithm != auth.PasswordAlgorithmBcrypt || token.AccessToken == "" {
		t.Fatalf("account=%+v token=%+v", account, token)
	}
	identity, err := authService.ValidateToken(ctx, token.AccessToken)
	if err != nil || identity.ReaderID != invitee {
		t.Fatalf("registered token identity=%+v err=%v", identity, err)
	}
	var used int
	if err := db.QueryRowContext(ctx, `SELECT used_count FROM reader_invite_codes WHERE id=$1`, inviteCodeID).Scan(&used); err != nil || used != 1 {
		t.Fatalf("invite used_count=%d err=%v", used, err)
	}
	var relationInviter, relationInvitee int64
	if err := db.QueryRowContext(ctx, `SELECT inviter_reader_id,invitee_reader_id FROM reader_invite_relations WHERE invitee_reader_id=$1`, invitee).Scan(&relationInviter, &relationInvitee); err != nil || relationInviter != inviter || relationInvitee != invitee {
		t.Fatalf("relation=%d/%d err=%v", relationInviter, relationInvitee, err)
	}
	var autoCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_invite_codes WHERE inviter_reader_id=$1`, invitee).Scan(&autoCount); err != nil || autoCount != 1 {
		t.Fatalf("automatic invite count=%d err=%v", autoCount, err)
	}
	var inviterBalance, inviteeBalance int64
	if err := db.QueryRowContext(ctx, `SELECT bonus_coin_balance FROM reader_wallets WHERE reader_id=$1`, inviter).Scan(&inviterBalance); err != nil || inviterBalance != 13 {
		t.Fatalf("inviter bonus=%d err=%v", inviterBalance, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT bonus_coin_balance FROM reader_wallets WHERE reader_id=$1`, invitee).Scan(&inviteeBalance); err != nil || inviteeBalance != 7 {
		t.Fatalf("invitee bonus=%d err=%v", inviteeBalance, err)
	}
	var rewardID, rewardRelationID, rewardInviterID, rewardInviteeID, rewardCoin int64
	var rewardStage, rewardStatus, rewardKey string
	if err := db.QueryRowContext(ctx, `SELECT id,relation_id,inviter_reader_id,invitee_reader_id,reward_stage,reward_coin,status,idempotency_key FROM reader_invite_reward_records WHERE invitee_reader_id=$1 AND reward_stage='register'`, invitee).Scan(&rewardID, &rewardRelationID, &rewardInviterID, &rewardInviteeID, &rewardStage, &rewardCoin, &rewardStatus, &rewardKey); err != nil {
		t.Fatal(err)
	}
	var inviterLedgerCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1 AND biz_type='invite_register_reward' AND biz_id=$2 AND idempotency_key=$3`, inviter, fmt.Sprintf("%d", rewardID), rewardKey).Scan(&inviterLedgerCount); err != nil {
		t.Fatal(err)
	}
	wantRewardKey := fmt.Sprintf("invite_reward:%d:register", invitee)
	if rewardRelationID <= 0 || rewardInviterID != inviter || rewardInviteeID != invitee || rewardStage != "register" || rewardCoin != 13 || rewardStatus != "granted" || rewardKey != wantRewardKey || inviterLedgerCount != 1 {
		t.Fatalf("reward=%d/%d/%d/%s/%d/%s/%s inviterLedgerCount=%d", rewardRelationID, rewardInviterID, rewardInviteeID, rewardStage, rewardCoin, rewardStatus, rewardKey, inviterLedgerCount)
	}
	var projectionUsername, projectionNickname, projectionStatus string
	if err := db.QueryRowContext(ctx, `SELECT username,nickname,status FROM commerce_reader_search_projection WHERE reader_id=$1`, invitee).Scan(&projectionUsername, &projectionNickname, &projectionStatus); err != nil || projectionUsername != inviteeName || projectionNickname != "被邀请人" || projectionStatus != "enabled" {
		t.Fatalf("projection=%q/%q/%q err=%v", projectionUsername, projectionNickname, projectionStatus, err)
	}
}

func TestConcurrentReaderRegistrationConsumesLimitedInviteOnce(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	inviterID, inviteCodeID := base, base+1
	code := fmt.Sprintf("MB-CONCURRENT-%d", base)
	usernamePrefix := fmt.Sprintf("itc-%d", base)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'fixture','enabled')`, inviterID, usernamePrefix+"-owner"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status,max_use_count) VALUES($1,$2,$3,'enabled',1)`, inviteCodeID, code, inviterID); err != nil {
		t.Fatal(err)
	}
	restoreRewardConfig := setInviteRewardConfig(t, db, ctx, false, 0, 0)
	t.Cleanup(restoreRewardConfig)
	t.Cleanup(func() {
		cleanup := context.Background()
		pattern := usernamePrefix + "-candidate-%"
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_sessions WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, pattern)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_invite_relations WHERE inviter_reader_id=$1 OR invitee_reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $2)`, inviterID, pattern)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_invite_codes WHERE id=$1 OR inviter_reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $2)`, inviteCodeID, pattern)
		_, _ = db.ExecContext(cleanup, `DELETE FROM commerce_reader_search_projection WHERE username LIKE $1`, pattern)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_accounts WHERE username LIKE $1 OR id=$2`, pattern, inviterID)
	})
	authService := auth.NewService(auth.SQLRepository{DB: db}, nil, auth.TokenConfig{Secret: []byte("integration-registration-secret-32"), TTL: time.Hour})
	service := NewService(SQLRepository{DB: db}, authService, transaction.New(db), commerceprovider.NewReaderSearch(db), commerceprovider.NewRegistrationReward(db))
	const workers = 8
	var successes atomic.Int64
	var wait sync.WaitGroup
	errorsByWorker := make(chan error, workers)
	for index := 0; index < workers; index++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			username := fmt.Sprintf("%s-candidate-%d", usernamePrefix, worker)
			if _, _, err := service.Register(ctx, auth.RegisterRequest{Username: username, Password: "invitee-password", InviteCode: code}); err == nil {
				successes.Add(1)
			} else {
				errorsByWorker <- err
			}
		}(index)
	}
	wait.Wait()
	close(errorsByWorker)
	if successes.Load() != 1 {
		failures := make([]string, 0, workers)
		for err := range errorsByWorker {
			failures = append(failures, err.Error())
		}
		t.Fatalf("successful registrations=%d failures=%v", successes.Load(), failures)
	}
	pattern := usernamePrefix + "-candidate-%"
	var usedCount, accounts, relations, projections, automaticCodes int
	if err := db.QueryRowContext(ctx, `SELECT used_count FROM reader_invite_codes WHERE id=$1`, inviteCodeID).Scan(&usedCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_accounts WHERE username LIKE $1`, pattern).Scan(&accounts); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_invite_relations WHERE inviter_reader_id=$1`, inviterID).Scan(&relations); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM commerce_reader_search_projection WHERE username LIKE $1`, pattern).Scan(&projections); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_invite_codes WHERE inviter_reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, pattern).Scan(&automaticCodes); err != nil {
		t.Fatal(err)
	}
	if usedCount != 1 || accounts != 1 || relations != 1 || projections != 1 || automaticCodes != 1 {
		t.Fatalf("facts used=%d accounts=%d relations=%d projections=%d automaticCodes=%d", usedCount, accounts, relations, projections, automaticCodes)
	}
}

type swallowedNestedReward struct{ transactor *transaction.Transactor }

func (reward swallowedNestedReward) GrantRegistrationRewards(ctx context.Context, _ commercecontract.RegistrationRewardRequest) error {
	_ = reward.transactor.Within(ctx, func(context.Context) error {
		return errors.New("forced nested reward failure")
	})
	return nil
}

func TestReaderRegistrationRollsBackSwallowedNestedRewardFailure(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	inviterID, inviteCodeID := base, base+1
	code := fmt.Sprintf("MB-NESTED-%d", base)
	username := fmt.Sprintf("%s-nested-%d", integrationtest.Prefix(), base)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'fixture','enabled')`, inviterID, username+"-owner"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status,max_use_count) VALUES($1,$2,$3,'enabled',1)`, inviteCodeID, code, inviterID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_sessions WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username=$1)`, username)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_invite_relations WHERE inviter_reader_id=$1`, inviterID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_invite_codes WHERE id=$1 OR inviter_reader_id IN (SELECT id FROM reader_accounts WHERE username=$2)`, inviteCodeID, username)
		_, _ = db.ExecContext(cleanup, `DELETE FROM commerce_reader_search_projection WHERE username=$1`, username)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_accounts WHERE username IN ($1,$2)`, username, username+"-owner")
	})
	transactor := transaction.New(db)
	authService := auth.NewService(auth.SQLRepository{DB: db}, nil, auth.TokenConfig{Secret: []byte("integration-registration-secret-32"), TTL: time.Hour})
	service := NewService(SQLRepository{DB: db}, authService, transactor, commerceprovider.NewReaderSearch(db), swallowedNestedReward{transactor: transactor})
	if _, _, err := service.Register(ctx, auth.RegisterRequest{Username: username, Password: "invitee-password", InviteCode: code}); !errors.Is(err, transaction.ErrRollbackOnly) {
		t.Fatalf("registration error=%v", err)
	}
	var accountCount, relationCount, projectionCount, usedCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_accounts WHERE username=$1`, username).Scan(&accountCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_invite_relations WHERE inviter_reader_id=$1`, inviterID).Scan(&relationCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM commerce_reader_search_projection WHERE username=$1`, username).Scan(&projectionCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT used_count FROM reader_invite_codes WHERE id=$1`, inviteCodeID).Scan(&usedCount); err != nil {
		t.Fatal(err)
	}
	if accountCount != 0 || relationCount != 0 || projectionCount != 0 || usedCount != 0 {
		t.Fatalf("rollback facts account=%d relation=%d projection=%d used=%d", accountCount, relationCount, projectionCount, usedCount)
	}
}

func setInviteRewardConfig(t *testing.T, db *sql.DB, ctx context.Context, enabled bool, inviterReward, inviteeReward int64) func() {
	t.Helper()
	var oldEnabled bool
	var oldInviterReward, oldInviteeReward int64
	var oldRemark string
	if err := db.QueryRowContext(ctx, `SELECT enabled,inviter_reward_coin,invitee_reward_coin,remark FROM reader_invite_reward_config WHERE id=1`).Scan(&oldEnabled, &oldInviterReward, &oldInviteeReward, &oldRemark); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE reader_invite_reward_config SET enabled=$1,inviter_reward_coin=$2,invitee_reward_coin=$3,remark='Reader 注册集成测试' WHERE id=1`, enabled, inviterReward, inviteeReward); err != nil {
		t.Fatal(err)
	}
	return func() {
		_, _ = db.ExecContext(context.Background(), `UPDATE reader_invite_reward_config SET enabled=$1,inviter_reward_coin=$2,invitee_reward_coin=$3,remark=$4 WHERE id=1`, oldEnabled, oldInviterReward, oldInviteeReward, oldRemark)
	}
}

type failingProjectionWriter struct{ err error }

func (writer failingProjectionWriter) UpsertReaderSearchProjection(context.Context, commercecontract.ReaderSearchProjection) error {
	return writer.err
}

func (writer failingProjectionWriter) DeleteReaderSearchProjection(context.Context, int64) error {
	return writer.err
}

func TestReaderRegistrationRollsBackWhenProjectionFails(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	inviterID, inviteCodeID := base, base+1
	code := fmt.Sprintf("MB-ROLLBACK-%d", base)
	username := integrationtest.Prefix() + "-rollback"
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'fixture','enabled')`, inviterID, integrationtest.Prefix()+"-inviter"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status,max_use_count) VALUES($1,$2,$3,'enabled',1)`, inviteCodeID, code, inviterID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_sessions WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username=$1)`, username)
		_, _ = db.ExecContext(cleanup, `DELETE FROM commerce_reader_search_projection WHERE username=$1`, username)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_invite_relations WHERE inviter_reader_id=$1`, inviterID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_invite_codes WHERE id=$1 OR inviter_reader_id IN (SELECT id FROM reader_accounts WHERE username=$2)`, inviteCodeID, username)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_accounts WHERE username=$1 OR id=$2`, username, inviterID)
	})
	authService := auth.NewService(auth.SQLRepository{DB: db}, nil, auth.TokenConfig{Secret: []byte("integration-registration-secret-32"), TTL: time.Hour})
	service := NewService(SQLRepository{DB: db}, authService, transaction.New(db), failingProjectionWriter{err: fmt.Errorf("forced projection failure")}, commerceprovider.NewRegistrationReward(db))

	if _, _, err := service.Register(ctx, auth.RegisterRequest{Username: username, Password: "invitee-password", InviteCode: code}); err == nil {
		t.Fatal("expected projection failure")
	}
	var accountCount, usedCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_accounts WHERE username=$1`, username).Scan(&accountCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT used_count FROM reader_invite_codes WHERE id=$1`, inviteCodeID).Scan(&usedCount); err != nil {
		t.Fatal(err)
	}
	if accountCount != 0 || usedCount != 0 {
		t.Fatalf("rollback accountCount=%d usedCount=%d", accountCount, usedCount)
	}
}
