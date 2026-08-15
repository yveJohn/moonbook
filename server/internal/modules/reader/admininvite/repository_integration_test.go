//go:build integration

package admininvite

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

func TestAdminInviteCrud(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r := SQLRepository{DB: db}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	v, err := r.Create(ctx, CreateInput{Code: "ADMIN-INVITE-" + suffix, MaxUseCount: "2", Remark: "创建备注"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_invite_codes WHERE id=$1`, v.ID)
	})
	if v.ID == "" || v.Code != "ADMIN-INVITE-"+suffix || v.MaxUseCount != "2" || v.Remark != "创建备注" {
		t.Fatalf("v=%+v", v)
	}
	rows, total, err := r.List(ctx, "ADMIN-INVITE-"+suffix, 1, 20)
	if err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("rows=%+v total=%d err=%v", rows, total, err)
	}
	if _, err = r.SetStatus(ctx, parseID(v.ID), "disabled"); err != nil {
		t.Fatal(err)
	}
	updated, err := r.Update(ctx, parseID(v.ID), UpdateInput{Code: "ADMIN-EDITED-" + suffix, Status: "enabled", MaxUseCount: "3", Remark: "编辑备注"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Code != "ADMIN-EDITED-"+suffix || updated.Status != "enabled" || updated.MaxUseCount != "3" || updated.Remark != "编辑备注" {
		t.Fatalf("updated=%+v", updated)
	}
	if err = r.Delete(ctx, parseID(v.ID)); err != nil {
		t.Fatal(err)
	}
}

func TestAdminInviteUpdateProtectsAutomaticCodeAndUsedCount(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	readerID, autoCodeID, manualCodeID := base, base+1, base+2
	suffix := fmt.Sprintf("%d", base)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash) VALUES($1,$2,'fixture')`, readerID, "invite-edit-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status,max_use_count,used_count,remark) VALUES($1,$2,$3,'enabled',NULL,0,'自动码'),($4,$5,NULL,'enabled',2,2,'人工码')`, autoCodeID, "AUTO-"+suffix, readerID, manualCodeID, "USED-"+suffix); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_invite_codes WHERE id IN ($1,$2)`, autoCodeID, manualCodeID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})
	r := SQLRepository{DB: db}
	auto, err := r.Update(ctx, autoCodeID, UpdateInput{Status: "disabled", MaxUseCount: "not-a-number", ExpiresAt: "not-a-time", Remark: "仅备注可改"})
	if err != nil {
		t.Fatal(err)
	}
	if auto.Code != "AUTO-"+suffix || auto.Status != "disabled" || auto.MaxUseCount != "" || auto.ExpiresAt != "" || auto.Remark != "仅备注可改" {
		t.Fatalf("auto=%+v", auto)
	}
	if err = r.Delete(ctx, autoCodeID); err == nil {
		t.Fatal("automatic invite code must not be deleted")
	} else if public := apperror.Expose(err); public.Code != apperror.CodeConflict || public.HTTPStatus != http.StatusConflict {
		t.Fatalf("automatic delete error=%+v", public)
	}
	if _, err = r.Update(ctx, manualCodeID, UpdateInput{Code: "USED-" + suffix, Status: "enabled", MaxUseCount: "1"}); err == nil {
		t.Fatal("max use count below used count must be rejected")
	}
	var maxUseCount int
	if err = db.QueryRowContext(ctx, `SELECT max_use_count FROM reader_invite_codes WHERE id=$1`, manualCodeID).Scan(&maxUseCount); err != nil || maxUseCount != 2 {
		t.Fatalf("maxUseCount=%d err=%v", maxUseCount, err)
	}
	if _, err = r.Update(ctx, manualCodeID, UpdateInput{Code: "AUTO-" + suffix, Status: "enabled", MaxUseCount: "2"}); err == nil {
		t.Fatal("duplicate invite code must be rejected")
	} else if public := apperror.Expose(err); public.Code != apperror.CodeConflict || public.HTTPStatus != http.StatusConflict {
		t.Fatalf("duplicate update error=%+v", public)
	}
}
func parseID(s string) int64 {
	var v int64
	for _, c := range s {
		v = v*10 + int64(c-'0')
	}
	return v
}
