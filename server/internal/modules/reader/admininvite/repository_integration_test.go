//go:build integration

package admininvite

import (
	"context"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"testing"
	"time"
)

func TestAdminInviteCrud(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r := SQLRepository{DB: db}
	v, err := r.Create(ctx, CreateInput{Code: "ADMIN-INVITE-FIXTURE", MaxUseCount: "2"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_invite_codes WHERE id=$1`, v.ID)
	})
	if v.ID == "" || v.Code != "ADMIN-INVITE-FIXTURE" || v.MaxUseCount != "2" {
		t.Fatalf("v=%+v", v)
	}
	rows, total, err := r.List(ctx, "ADMIN-INVITE-FIXTURE", 1, 20)
	if err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("rows=%+v total=%d err=%v", rows, total, err)
	}
	if _, err = r.SetStatus(ctx, parseID(v.ID), "disabled"); err != nil {
		t.Fatal(err)
	}
	if err = r.Delete(ctx, parseID(v.ID)); err != nil {
		t.Fatal(err)
	}
}
func parseID(s string) int64 {
	var v int64
	for _, c := range s {
		v = v*10 + int64(c-'0')
	}
	return v
}
