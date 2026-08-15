//go:build integration

package provider

import (
	"context"
	"fmt"
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
