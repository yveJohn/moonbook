//go:build integration

package auth

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestReaderAuthPostgresRedisLegacyMD5AndRateLimit(t *testing.T) {
	db, cfg := integrationtest.RequireDB(t)
	rdb := integrationtest.RequireRedis(t, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	id := time.Now().UnixNano()
	username := integrationtest.Prefix()
	sum := md5.Sum([]byte("legacy-password"))
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash,password_algorithm,status) VALUES($1,$2,'集成测试',$3,'md5','enabled')`, id, username, hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_sessions WHERE reader_id=$1`, id)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, id)
	})
	limiter := RedisRateLimiter{Client: rdb, Prefix: "moonbook:reader-it:" + fmt.Sprint(id) + ":"}
	service := NewService(SQLRepository{DB: db}, limiter, TokenConfig{Secret: []byte("integration-reader-secret-32-bytes-long"), TTL: time.Hour, Issuer: "moonbook-reader-integration"})
	token, err := service.Login(ctx, username, "legacy-password", "198.51.100.10")
	if err != nil {
		t.Fatalf("legacy login: %v", err)
	}
	identity, err := service.ValidateToken(ctx, token.AccessToken)
	if err != nil || identity.ReaderID != id {
		t.Fatalf("validate identity=%+v err=%v", identity, err)
	}
	var algorithm, hash string
	if err := db.QueryRowContext(ctx, `SELECT password_algorithm,password_hash FROM reader_accounts WHERE id=$1`, id).Scan(&algorithm, &hash); err != nil {
		t.Fatal(err)
	}
	if algorithm != PasswordAlgorithmBcrypt || hash == hex.EncodeToString(sum[:]) {
		t.Fatalf("password was not upgraded: algorithm=%s", algorithm)
	}
	if err := service.Logout(ctx, token.AccessToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := service.ValidateToken(ctx, token.AccessToken); err == nil {
		t.Fatal("revoked reader token remained valid")
	}
	for i := 0; i < 12; i++ {
		_, err = service.Login(ctx, username, "wrong-password", "198.51.100.11")
	}
	if err != ErrRateLimited {
		t.Fatalf("rate limit result=%v, want ErrRateLimited", err)
	}
	var sessions int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_sessions WHERE reader_id=$1`, id).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 1 {
		t.Fatalf("failed/rate-limited logins created %d sessions", sessions)
	}
}
