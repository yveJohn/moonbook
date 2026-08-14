//go:build integration

package admininvitereward

import (
	"context"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestInviteRewardConfigCRUD(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	r := SQLRepository{DB: db}
	old, err := r.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `UPDATE reader_invite_reward_config SET enabled=$1,inviter_reward_coin=$2,invitee_reward_coin=$3,remark=$4 WHERE id=1`, old.Enabled == "true", old.InviterRewardCoin, old.InviteeRewardCoin, old.Remark)
	})
	v, err := r.Update(ctx, Input{Enabled: true, InviterRewardCoin: "12", InviteeRewardCoin: "8", Remark: "集成配置"})
	if err != nil || v.ID != "1" || v.Enabled != "true" || v.InviterRewardCoin != "12" || v.InviteeRewardCoin != "8" {
		t.Fatalf("config=%+v err=%v", v, err)
	}
	got, err := r.Get(ctx)
	if err != nil || got.Remark != "集成配置" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}
