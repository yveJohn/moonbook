//go:build integration

package invite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
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
	if _, err = db.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status,max_use_count) VALUES($1,$2,$3,'enabled',1)`, inviteCodeID, code, inviter); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		for _, q := range []string{`DELETE FROM reader_sessions WHERE reader_id IN ($1,$2)`, `DELETE FROM reader_invite_relations WHERE inviter_reader_id IN ($1,$2) OR invitee_reader_id IN ($1,$2)`, `DELETE FROM reader_accounts WHERE id IN ($1,$2)`} {
			_, _ = db.ExecContext(cleanup, q, inviter, invitee)
		}
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_invite_codes WHERE id=$3 OR inviter_reader_id IN ($1,$2)`, inviter, invitee, inviteCodeID)
	})
	authService := auth.NewService(auth.SQLRepository{DB: db}, nil, auth.TokenConfig{Secret: []byte("integration-registration-secret-32"), TTL: time.Hour, Issuer: "moonbook-reader-registration"})
	registration := NewService(SQLRepository{DB: db}, authService)
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
}
