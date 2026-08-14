//go:build integration

package adminuser

import (
	"context"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"golang.org/x/crypto/bcrypt"
	"testing"
	"time"
)

func TestAdminUserListGetAndDisableRevokesSessions(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var id int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),2607300000000)+1 FROM reader_accounts`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash,status) VALUES($1,'admin-user-fixture','测试读者','fixture','enabled')`, id); err != nil {
		t.Fatal(err)
	}
	var session int64
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_sessions(reader_id,token_digest,expires_at) VALUES($1,repeat('a',64),now()+interval '1 hour') RETURNING id`, id).Scan(&session); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_sessions WHERE reader_id=$1`, id)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, id)
	})
	r := SQLRepository{DB: db}
	users, total, err := r.List(ctx, "admin-user-fixture", "enabled", 1, 20)
	if err != nil || total != 1 || len(users) != 1 || users[0].ID == "" {
		t.Fatalf("users=%+v total=%d err=%v", users, total, err)
	}
	if _, err = r.SetStatus(ctx, id, "disabled"); err != nil {
		t.Fatal(err)
	}
	var revoked bool
	if err = db.QueryRowContext(ctx, `SELECT revoked_at IS NOT NULL FROM reader_sessions WHERE id=$1`, session).Scan(&revoked); err != nil || !revoked {
		t.Fatalf("revoked=%v err=%v", revoked, err)
	}
}

func TestAdminUserResetPasswordRevokesSessions(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var id int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),2607300000000)+1 FROM reader_accounts`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,password_algorithm,status) VALUES($1,'admin-reset-fixture','old-hash','md5','enabled')`, id); err != nil {
		t.Fatal(err)
	}
	var session int64
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_sessions(reader_id,token_digest,expires_at) VALUES($1,repeat('b',64),now()+interval '1 hour') RETURNING id`, id).Scan(&session); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_sessions WHERE reader_id=$1`, id)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, id)
	})
	if err := (SQLRepository{DB: db}).ResetPassword(ctx, id, "new-pass-123", "new-pass-123"); err != nil {
		t.Fatal(err)
	}
	var hash, algorithm string
	if err := db.QueryRowContext(ctx, `SELECT password_hash,password_algorithm FROM reader_accounts WHERE id=$1`, id).Scan(&hash, &algorithm); err != nil {
		t.Fatal(err)
	}
	if algorithm != "bcrypt" || bcrypt.CompareHashAndPassword([]byte(hash), []byte("new-pass-123")) != nil {
		t.Fatalf("algorithm=%s hash invalid", algorithm)
	}
	var revoked bool
	if err := db.QueryRowContext(ctx, `SELECT revoked_at IS NOT NULL FROM reader_sessions WHERE id=$1`, session).Scan(&revoked); err != nil || !revoked {
		t.Fatalf("revoked=%v err=%v", revoked, err)
	}
	if err := (SQLRepository{DB: db}).ResetPassword(ctx, id, "short", "short"); err == nil {
		t.Fatal("expected short password rejection")
	}
}
