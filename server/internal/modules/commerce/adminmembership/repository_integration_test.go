//go:build integration

package adminmembership

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestAdminMembershipGrantIsIdempotentAndAdditive(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var readerID int64
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),2607300006000)+1 FROM reader_accounts`).Scan(&readerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'fixture','enabled')`, readerID, "membership-admin-"+suffix); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM commerce_membership_grants WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})
	r := SQLRepository{DB: db}
	first, err := r.Grant(ctx, readerID, Input{RequestID: "grant-" + suffix, DurationDays: "30", Remark: "活动赠送"})
	if err != nil || first.ID == "" || first.Permanent != "false" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	retry, err := r.Grant(ctx, readerID, Input{RequestID: "grant-" + suffix, DurationDays: "30", Remark: "重复请求"})
	if err != nil || retry.ID != first.ID {
		t.Fatalf("retry=%+v err=%v", retry, err)
	}
	second, err := r.Grant(ctx, readerID, Input{RequestID: "grant-2-" + suffix, Permanent: true, Remark: "永久赠送"})
	if err != nil || second.ID == first.ID || second.Permanent != "true" {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM commerce_membership_grants WHERE reader_id=$1 AND source_type='admin'`, readerID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}
