//go:build integration

package adminoperations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestOperationsAuditPreservesLongIDsAndMissingProjection(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var base int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),9007199254743000)+100 FROM reader_accounts`).Scan(&base); err != nil {
		t.Fatal(err)
	}
	inviter, invitee, missing := base, base+1, base+2
	codeID, relationID := base+3, base+4
	name := fmt.Sprintf("operations-fixture-%d", inviter)
	_, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash) VALUES($1,$2,'Inviter','fixture'),($3,$4,'Invitee','fixture'),($5,$6,'','fixture')`, inviter, name, invitee, name+"-invitee", missing, name+"-missing")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO commerce_reader_search_projection(reader_id,username,nickname,status) VALUES($1,$2,'Inviter','enabled'),($3,$4,'Invitee','enabled')`, inviter, name, invitee, name+"-invitee")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id) VALUES($1,$2,$3)`, codeID, "operations-"+fmt.Sprint(codeID), inviter)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO reader_invite_relations(id,inviter_reader_id,invitee_reader_id,invite_code_id) VALUES($1,$2,$3,$4)`, relationID, inviter, invitee, codeID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO reader_checkin_records(id,reader_id,checkin_date,continuous_days,base_reward_coin,total_reward_coin,idempotency_key) VALUES($1,$2,'2026-08-20',3,12,12,$3),($4,$5,'2026-08-19',1,8,8,$6)`, base+5, inviter, "operations-checkin-1", base+6, missing, "operations-checkin-2")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO reader_invite_reward_records(id,relation_id,inviter_reader_id,invitee_reader_id,reward_stage,reward_coin,status,idempotency_key,granted_at) VALUES($1,$2,$3,$4,'first_recharge',99,'granted',$5,'2026-08-20T12:00:00Z')`, base+7, relationID, inviter, invitee, "operations-reward-1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_invite_reward_records WHERE id=$1`, base+7)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_checkin_records WHERE id IN ($1,$2)`, base+5, base+6)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_invite_relations WHERE id=$1`, relationID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_invite_codes WHERE id=$1`, codeID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM commerce_reader_search_projection WHERE reader_id IN ($1,$2)`, inviter, invitee)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_accounts WHERE id IN ($1,$2,$3)`, inviter, invitee, missing)
	})
	r := SQLRepository{DB: db}
	checkins, total, err := r.ListCheckins(ctx, CheckinFilter{ReaderKeyword: name, Page: 1, PageSize: 20})
	if err != nil || total != 1 || len(checkins) != 1 || checkins[0].ID != fmt.Sprint(base+5) || checkins[0].ReaderID != fmt.Sprint(inviter) {
		t.Fatalf("checkins=%+v total=%d err=%v", checkins, total, err)
	}
	allCheckins, total, err := r.ListCheckins(ctx, CheckinFilter{Page: 1, PageSize: 20})
	if err != nil || total < 2 || len(allCheckins) < 2 {
		t.Fatalf("all checkins=%+v total=%d err=%v", allCheckins, total, err)
	}
	for _, row := range allCheckins {
		if row.ReaderID == fmt.Sprint(missing) && (row.Username != "" || row.Nickname != "") {
			t.Fatalf("missing projection was populated: %+v", row)
		}
	}
	rewards, total, err := r.ListInviteRewards(ctx, InviteRewardFilter{InviterKeyword: name, RewardStage: "first_recharge", Status: "granted", Page: 1, PageSize: 20})
	if err != nil || total != 1 || len(rewards) != 1 || rewards[0].ID != fmt.Sprint(base+7) || rewards[0].RewardCoin != "99" {
		t.Fatalf("rewards=%+v total=%d err=%v", rewards, total, err)
	}
}
