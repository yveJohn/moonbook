//go:build integration

package adminuser

import (
	"context"
	"fmt"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	commerceprovider "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/provider"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/wallet"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
	"golang.org/x/crypto/bcrypt"
	"strings"
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
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_wallets(reader_id,recharge_coin_balance,bonus_coin_balance) VALUES($1,88,15)`, id); err != nil {
		t.Fatal(err)
	}
	var session int64
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_sessions(reader_id,token_digest,expires_at) VALUES($1,repeat('a',64),now()+interval '1 hour') RETURNING id`, id).Scan(&session); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_sessions WHERE reader_id=$1`, id)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_wallets WHERE reader_id=$1`, id)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM commerce_reader_search_projection WHERE reader_id=$1`, id)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, id)
	})
	r := SQLRepository{DB: db}
	service := NewService(r, transaction.New(db), commerceprovider.NewReaderSearch(db), nil, commerceprovider.NewWallet(wallet.NewService(wallet.SQLRepository{DB: db})))
	users, total, err := service.List(ctx, "admin-user-fixture", "enabled", 1, 20)
	if err != nil || total != 1 || len(users) != 1 || users[0].ID == "" {
		t.Fatalf("users=%+v total=%d err=%v", users, total, err)
	}
	if users[0].RechargeBalance != "88" || users[0].BonusBalance != "15" {
		t.Fatalf("balances recharge=%q bonus=%q", users[0].RechargeBalance, users[0].BonusBalance)
	}
	if _, err = service.SetStatus(ctx, id, "disabled"); err != nil {
		t.Fatal(err)
	}
	var revoked bool
	if err = db.QueryRowContext(ctx, `SELECT revoked_at IS NOT NULL FROM reader_sessions WHERE id=$1`, session).Scan(&revoked); err != nil || !revoked {
		t.Fatalf("revoked=%v err=%v", revoked, err)
	}
	var projectionStatus string
	if err = db.QueryRowContext(ctx, `SELECT status FROM commerce_reader_search_projection WHERE reader_id=$1`, id).Scan(&projectionStatus); err != nil || projectionStatus != "disabled" {
		t.Fatalf("projection status=%q err=%v", projectionStatus, err)
	}
}

type failingStatusProjection struct{ err error }

func (writer failingStatusProjection) UpsertReaderSearchProjection(context.Context, commercecontract.ReaderSearchProjection) error {
	return writer.err
}

func (writer failingStatusProjection) DeleteReaderSearchProjection(context.Context, int64) error {
	return writer.err
}

func TestAdminUserListReturnsZeroBalancesWithoutWallet(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var id int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),2607300000000)+1 FROM reader_accounts`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	username := fmt.Sprintf("admin-user-nowallet-%d", id)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash,status) VALUES($1,$2,'无钱包','fixture','enabled')`, id, username); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, id)
	})
	users, total, err := NewService(SQLRepository{DB: db}, transaction.New(db), commerceprovider.NewReaderSearch(db), nil, commerceprovider.NewWallet(wallet.NewService(wallet.SQLRepository{DB: db}))).List(ctx, username, "enabled", 1, 20)
	if err != nil || total != 1 || len(users) != 1 {
		t.Fatalf("users=%+v total=%d err=%v", users, total, err)
	}
	if users[0].RechargeBalance != "0" || users[0].BonusBalance != "0" {
		t.Fatalf("balances recharge=%q bonus=%q", users[0].RechargeBalance, users[0].BonusBalance)
	}
}

func TestAdminUserStatusRollsBackWhenProjectionFails(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var id int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),2607300000000)+1 FROM reader_accounts`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	username := fmt.Sprintf("admin-user-rollback-%d", id)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'fixture','enabled')`, id, username); err != nil {
		t.Fatal(err)
	}
	var session int64
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_sessions(reader_id,token_digest,expires_at) VALUES($1,repeat('c',64),now()+interval '1 hour') RETURNING id`, id).Scan(&session); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_sessions WHERE reader_id=$1`, id)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM commerce_reader_search_projection WHERE reader_id=$1`, id)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, id)
	})
	service := NewService(SQLRepository{DB: db}, transaction.New(db), failingStatusProjection{err: fmt.Errorf("forced projection failure")}, nil, nil)
	if _, err := service.SetStatus(ctx, id, "disabled"); err == nil {
		t.Fatal("expected projection failure")
	}
	var status string
	var revoked bool
	if err := db.QueryRowContext(ctx, `SELECT status FROM reader_accounts WHERE id=$1`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT revoked_at IS NOT NULL FROM reader_sessions WHERE id=$1`, session).Scan(&revoked); err != nil {
		t.Fatal(err)
	}
	if status != "enabled" || revoked {
		t.Fatalf("status=%q revoked=%t", status, revoked)
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
	username := fmt.Sprintf("admin-reset-fixture-%d", id)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,password_algorithm,status) VALUES($1,$2,'old-hash','md5','enabled')`, id, username); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_sessions WHERE reader_id=$1`, id)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM commerce_reader_search_projection WHERE reader_id=$1`, id)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, id)
	})
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_reader_search_projection(reader_id,username,nickname,status) VALUES($1,$2,'','enabled')`, id, username); err != nil {
		t.Fatal(err)
	}
	var session int64
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_sessions(reader_id,token_digest,expires_at) VALUES($1,repeat('b',64),now()+interval '1 hour') RETURNING id`, id).Scan(&session); err != nil {
		t.Fatal(err)
	}
	var projectionUpdatedAt time.Time
	if err := db.QueryRowContext(ctx, `SELECT updated_at FROM commerce_reader_search_projection WHERE reader_id=$1`, id).Scan(&projectionUpdatedAt); err != nil {
		t.Fatal(err)
	}
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
	var projectionUpdatedAfter time.Time
	if err := db.QueryRowContext(ctx, `SELECT updated_at FROM commerce_reader_search_projection WHERE reader_id=$1`, id).Scan(&projectionUpdatedAfter); err != nil || !projectionUpdatedAfter.Equal(projectionUpdatedAt) {
		t.Fatalf("projection updated_at before=%s after=%s err=%v", projectionUpdatedAt, projectionUpdatedAfter, err)
	}
	if err := (SQLRepository{DB: db}).ResetPassword(ctx, id, "short", "short"); err == nil {
		t.Fatal("expected short password rejection")
	}
}

func TestAdminUserListOperationsIsolatesReaderAndRedactsPassword(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var id int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),2607300000000)+1 FROM reader_accounts`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	username := fmt.Sprintf("admin-user-ops-%d", id)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash,status) VALUES($1,$2,'日志读者','fixture','enabled')`, id, username); err != nil {
		t.Fatal(err)
	}
	neighbor := id + 1
	secret := "secret-pass-xyz"
	records := []struct {
		method, path, body string
		status             int
		deleted            bool
	}{
		{"PUT", fmt.Sprintf("/reader/users/%d/password", id), fmt.Sprintf(`{"password":%q,"confirmPassword":%q}`, secret, secret), 200, false},
		{"PUT", fmt.Sprintf("/reader/users/%d/status", id), `{"status":"disabled"}`, 200, false},
		{"POST", fmt.Sprintf("/reader/users/%d/membership", id), `{"productId":"88","remark":"补偿会员"}`, 200, false},
		{"POST", fmt.Sprintf("/reader/wallets/%d/adjust", id), `{"coinType":"bonus","direction":"income","amount":"9","reason":"补偿金币"}`, 200, false},
		{"PUT", fmt.Sprintf("/api/reader/users/%d/status", id), `{"status":"enabled"}`, 200, false},
		{"PUT", fmt.Sprintf("/reader/users/%d/status", neighbor), `{"status":"disabled"}`, 200, false},
		{"PUT", fmt.Sprintf("/reader/users/%d0/status", id), `{"status":"disabled"}`, 200, false},
		{"PUT", fmt.Sprintf("/reader/users/%d/status", id), `{"status":"enabled"}`, 200, true},
	}
	ids := make([]int64, 0, len(records))
	for _, record := range records {
		var recID int64
		err := db.QueryRowContext(ctx, `INSERT INTO sys_operation_records(created_at,updated_at,deleted_at,ip,method,path,status,body,resp,user_id) VALUES(now(),now(),CASE WHEN $1 THEN now() ELSE NULL END,'127.0.0.1',$2,$3,$4,$5,'',NULL) RETURNING id`, record.deleted, record.method, record.path, record.status, record.body).Scan(&recID)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, recID)
	}
	t.Cleanup(func() {
		for _, recID := range ids {
			_, _ = db.ExecContext(context.Background(), `DELETE FROM sys_operation_records WHERE id=$1`, recID)
		}
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, id)
	})
	service := NewService(SQLRepository{DB: db}, transaction.New(db), commerceprovider.NewReaderSearch(db), nil, commerceprovider.NewWallet(wallet.NewService(wallet.SQLRepository{DB: db})))
	rows, total, err := service.ListOperations(ctx, id, 1, 20)
	if err != nil || total != 5 || len(rows) != 5 {
		t.Fatalf("rows=%+v total=%d err=%v", rows, total, err)
	}
	encoded := fmt.Sprintf("%+v", rows)
	if strings.Contains(encoded, secret) || strings.Contains(encoded, "confirmPassword") {
		t.Fatalf("password leaked: %s", encoded)
	}
	actions := map[string]int{}
	for _, row := range rows {
		if row.Body != "" {
			t.Fatalf("raw body returned: %+v", row)
		}
		if strings.Contains(row.Path, fmt.Sprintf("/users/%d0/", id)) || strings.Contains(row.Path, fmt.Sprintf("/users/%d/", neighbor)) {
			t.Fatalf("neighbor path leaked: %+v", row)
		}
		actions[row.Action]++
		if row.ID == "" {
			t.Fatal("operation id must be a string")
		}
	}
	if actions["重置密码"] != 1 || actions["启停账号"] != 2 || actions["发放会员"] != 1 || actions["发放金币"] != 1 {
		t.Fatalf("actions=%v rows=%+v", actions, rows)
	}
}
