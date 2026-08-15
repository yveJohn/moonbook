//go:build integration

package invite

import (
	"context"
	"fmt"
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
	if _, err = db.ExecContext(ctx, `UPDATE reader_invite_reward_config SET enabled=true,inviter_reward_coin=13,invitee_reward_coin=7,remark='集成测试' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `UPDATE reader_invite_reward_config SET enabled=false,inviter_reward_coin=0,invitee_reward_coin=0,remark='' WHERE id=1`)
	})
	if _, err = db.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status,max_use_count) VALUES($1,$2,$3,'enabled',1)`, inviteCodeID, code, inviter); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_sessions WHERE reader_id IN ($1,$2)`, inviter, invitee)
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
	var projectionUsername, projectionNickname, projectionStatus string
	if err := db.QueryRowContext(ctx, `SELECT username,nickname,status FROM commerce_reader_search_projection WHERE reader_id=$1`, invitee).Scan(&projectionUsername, &projectionNickname, &projectionStatus); err != nil || projectionUsername != inviteeName || projectionNickname != "被邀请人" || projectionStatus != "enabled" {
		t.Fatalf("projection=%q/%q/%q err=%v", projectionUsername, projectionNickname, projectionStatus, err)
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
