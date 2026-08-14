//go:build integration

package admincheckin

import (
	"context"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestAdminCheckinRuleCRUDAndConstraints(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r := SQLRepository{DB: db}
	v, err := r.Create(ctx, Input{RuleType: "continuous", ContinuousDays: "99", RewardMode: "random", MinCoin: "3", MaxCoin: "8", Status: "disabled", SortOrder: "999", Remark: "integration"})
	if err != nil {
		t.Fatal(err)
	}
	id := parseRuleID(v.ID)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_checkin_reward_rules WHERE id=$1`, id)
	})
	if v.ID == "" || v.ContinuousDays != "99" || v.MinCoin != "3" || v.MaxCoin != "8" {
		t.Fatalf("created rule = %+v", v)
	}
	rows, total, err := r.List(ctx, "continuous", 1, 20)
	if err != nil || total < 1 || len(rows) == 0 {
		t.Fatalf("list rows=%+v total=%d err=%v", rows, total, err)
	}
	v, err = r.Update(ctx, id, Input{RuleType: "continuous", ContinuousDays: "99", RewardMode: "fixed", FixedCoin: "12", Status: "enabled", SortOrder: "1000", Remark: "updated"})
	if err != nil || v.Status != "enabled" || v.FixedCoin != "12" {
		t.Fatalf("updated rule=%+v err=%v", v, err)
	}
	_, err = r.Create(ctx, Input{RuleType: "continuous", ContinuousDays: "99", RewardMode: "fixed", FixedCoin: "20", Status: "enabled"})
	if err == nil {
		t.Fatal("expected enabled continuous uniqueness violation")
	}
	if err := r.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := valid(Input{RuleType: "daily", RewardMode: "fixed", FixedCoin: "", Status: "enabled"}); err == nil {
		t.Fatal("expected fixed reward validation error")
	}
	if err := valid(Input{RuleType: "daily", RewardMode: "random", MinCoin: "10", MaxCoin: "2", Status: "enabled"}); err == nil {
		t.Fatal("expected random reward range validation error")
	}
}

func parseRuleID(s string) int64 {
	var v int64
	for _, c := range s {
		v = v*10 + int64(c-'0')
	}
	return v
}
