//go:build integration

package provider

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestAccountSummaryReadsCommerceOwnedFacts(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	readerID, entitlementID, membershipID, bookID := base, base+1, base+2, base+3
	username := fmt.Sprintf("account-summary-it-%d", base)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash,status) VALUES($1,$2,$2,'x','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_entitlements(id,reader_id,entitlement_type,target_id,starts_at,permanent,status) VALUES($1,$2,'book',$3,now(),true,'active')`, entitlementID, readerID, bookID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_membership_grants(id,reader_id,grant_type,starts_at,permanent,status) VALUES($1,$2,'admin',now(),true,'active')`, membershipID, readerID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = db.ExecContext(cleanup, `DELETE FROM commerce_entitlements WHERE id=$1`, entitlementID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM commerce_membership_grants WHERE id=$1`, membershipID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})

	provider := NewAccountSummary(db)
	summary, err := provider.Entitlements(ctx, readerID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.ReaderID != readerID || len(summary.BookIDs) != 1 || summary.BookIDs[0] != bookID || !summary.MembershipActive || !summary.MembershipPermanent || summary.MembershipExpiresAt != nil {
		t.Fatalf("summary=%+v", summary)
	}
	if _, err := provider.MembershipProducts(ctx); err != nil {
		t.Fatalf("membership products: %v", err)
	}
	rewards, err := provider.InviteRewardSummary(ctx, readerID)
	if err != nil || rewards.FirstRechargeRewardCoin != 100 || rewards.TotalRewardCoin != 0 {
		t.Fatalf("rewards=%+v err=%v", rewards, err)
	}
}

func TestInviteRewardSummaryReadsInviterFacts(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	base := time.Now().UnixNano()
	inviterID, otherInviterID := base, base+1
	inviterCodeID, otherCodeID := base+10, base+11
	fixed := time.Date(2026, 9, 1, 1, 0, 0, 0, time.UTC)

	var oldEnabled bool
	var oldInviterReward, oldInviteeReward int64
	if err := db.QueryRowContext(ctx, `SELECT enabled,inviter_reward_coin,invitee_reward_coin FROM reader_invite_reward_config WHERE id=1`).Scan(&oldEnabled, &oldInviterReward, &oldInviteeReward); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE reader_invite_reward_config SET enabled=true,inviter_reward_coin=30,invitee_reward_coin=7 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash,status) VALUES($1,$2,'','fixture','enabled'),($3,$4,'','fixture','enabled')`, inviterID, "reward-summary-inviter-"+strconv.FormatInt(base, 10), otherInviterID, "reward-summary-other-"+strconv.FormatInt(base, 10)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status) VALUES($1,$2,$3,'enabled'),($4,$5,$6,'enabled')`, inviterCodeID, "RSI-"+strconv.FormatInt(base, 10), inviterID, otherCodeID, "RSO-"+strconv.FormatInt(base, 10), otherInviterID); err != nil {
		t.Fatal(err)
	}

	for index := int64(0); index < 24; index++ {
		inviteeID := base + 100 + index
		relationID := base + 200 + index
		rewardID := base + 300 + index
		username := fmt.Sprintf("reward-summary-invitee-%d-%d", base, index)
		if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash,status) VALUES($1,$2,'','fixture','enabled')`, inviteeID, username); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_relations(id,inviter_reader_id,invitee_reader_id,invite_code_id,status) VALUES($1,$2,$3,$4,'active')`, relationID, inviterID, inviteeID, inviterCodeID); err != nil {
			t.Fatal(err)
		}
		status := "granted"
		amount := index + 1
		var grantedAt any = fixed.Add(time.Duration(index) * time.Minute)
		if index == 22 {
			grantedAt = nil
		}
		if index == 23 {
			status = "failed"
			amount = 999
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_reward_records(id,relation_id,inviter_reader_id,invitee_reader_id,reward_stage,reward_coin,status,idempotency_key,granted_at,remark) VALUES($1,$2,$3,$4,'register',$5,$6,$7,$8,'汇总测试')`, rewardID, relationID, inviterID, inviteeID, amount, status, fmt.Sprintf("reward-summary-%d-%d", base, index), grantedAt); err != nil {
			t.Fatal(err)
		}
	}

	otherRelationID, otherRewardID := base+500, base+501
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_relations(id,inviter_reader_id,invitee_reader_id,invite_code_id,status) VALUES($1,$2,$3,$4,'active')`, otherRelationID, otherInviterID, inviterID, otherCodeID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_reward_records(id,relation_id,inviter_reader_id,invitee_reader_id,reward_stage,reward_coin,status,idempotency_key,granted_at,remark) VALUES($1,$2,$3,$4,'first_recharge',888,'granted',$5,$6,'不应计入')`, otherRewardID, otherRelationID, otherInviterID, inviterID, fmt.Sprintf("reward-summary-other-%d", base), fixed.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_invite_reward_records WHERE inviter_reader_id IN ($1,$2)`, inviterID, otherInviterID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_invite_relations WHERE inviter_reader_id IN ($1,$2)`, inviterID, otherInviterID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_invite_codes WHERE id IN ($1,$2)`, inviterCodeID, otherCodeID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_accounts WHERE id BETWEEN $1 AND $2 OR id IN ($3,$4)`, base+100, base+123, inviterID, otherInviterID)
		_, _ = db.ExecContext(cleanup, `UPDATE reader_invite_reward_config SET enabled=$1,inviter_reward_coin=$2,invitee_reward_coin=$3 WHERE id=1`, oldEnabled, oldInviterReward, oldInviteeReward)
	})

	summary, err := NewAccountSummary(db).InviteRewardSummary(ctx, inviterID)
	if err != nil {
		t.Fatal(err)
	}
	if summary.RegisterRewardCoin != 30 || summary.FirstRechargeRewardCoin != 100 || summary.TotalRewardCoin != 276 {
		t.Fatalf("reward totals=%+v", summary)
	}
	if len(summary.Records) != 20 {
		t.Fatalf("record count=%d", len(summary.Records))
	}
	if summary.Records[0].ID != base+321 || summary.Records[0].RewardCoin != 22 || summary.Records[19].ID != base+302 || summary.Records[19].RewardCoin != 3 {
		t.Fatalf("unexpected ordered records first=%+v last=%+v", summary.Records[0], summary.Records[19])
	}
	for _, record := range summary.Records {
		if record.ID == otherRewardID || record.RewardCoin == 888 || record.RewardCoin == 999 || record.GrantedAt == nil {
			t.Fatalf("unexpected filtered record=%+v", record)
		}
	}
}
