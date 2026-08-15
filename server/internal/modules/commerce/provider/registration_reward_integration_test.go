//go:build integration

package provider

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

func TestRegistrationRewardIsConcurrentAndIdempotent(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	inviterID, inviteeID, relationID := base, base+1, base+2
	prefix := fmt.Sprintf("registration-reward-it-%d", base)
	restoreRewardConfig := setRegistrationRewardConfig(t, db, ctx, true, 13, 7)
	t.Cleanup(restoreRewardConfig)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'fixture','enabled'),($3,$4,'fixture','enabled')`, inviterID, prefix+"-inviter", inviteeID, prefix+"-invitee"); err != nil {
		t.Fatal(err)
	}
	cleanupRegistrationRewardFacts(t, db, []int64{inviterID, inviteeID})

	provider := NewRegistrationReward(db)
	transactor := transaction.New(db)
	request := commercecontract.RegistrationRewardRequest{RelationID: relationID, InviterID: inviterID, InviteeID: inviteeID}
	const workers = 8
	errorsByWorker := make(chan error, workers)
	var wait sync.WaitGroup
	for index := 0; index < workers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			errorsByWorker <- transactor.Within(ctx, func(txCtx context.Context) error {
				return provider.GrantRegistrationRewards(txCtx, request)
			})
		}()
	}
	wait.Wait()
	close(errorsByWorker)
	for err := range errorsByWorker {
		if err != nil {
			t.Fatalf("concurrent reward: %v", err)
		}
	}
	assertRegistrationRewardFacts(t, db, ctx, inviterID, inviteeID, 13, 7, 2)
}

func TestRegistrationRewardRollsBackInviterWhenInviteeLedgerFails(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	inviterID, inviteeID, blockerID, relationID := base, base+1, base+2, base+3
	prefix := fmt.Sprintf("registration-reward-rollback-%d", base)
	restoreRewardConfig := setRegistrationRewardConfig(t, db, ctx, true, 13, 7)
	t.Cleanup(restoreRewardConfig)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'fixture','enabled'),($3,$4,'fixture','enabled'),($5,$6,'fixture','enabled')`, inviterID, prefix+"-inviter", inviteeID, prefix+"-invitee", blockerID, prefix+"-blocker"); err != nil {
		t.Fatal(err)
	}
	cleanupRegistrationRewardFacts(t, db, []int64{inviterID, inviteeID, blockerID})
	businessID := fmt.Sprintf("%d:%d", relationID, inviteeID)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_wallets(reader_id) VALUES($1)`, blockerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_wallet_ledgers(reader_id,ledger_no,biz_type,direction,coin_type,amount,balance_before,balance_after) VALUES($1,$2,'fixture','income','bonus',1,0,1)`, blockerID, "INVE-"+businessID); err != nil {
		t.Fatal(err)
	}

	provider := NewRegistrationReward(db)
	err := transaction.New(db).Within(ctx, func(txCtx context.Context) error {
		return provider.GrantRegistrationRewards(txCtx, commercecontract.RegistrationRewardRequest{RelationID: relationID, InviterID: inviterID, InviteeID: inviteeID})
	})
	if err == nil {
		t.Fatal("expected invitee ledger conflict")
	}
	var inviterWallets, inviterLedgers, inviteeWallets int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallets WHERE reader_id=$1`, inviterID).Scan(&inviterWallets); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1`, inviterID).Scan(&inviterLedgers); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallets WHERE reader_id=$1`, inviteeID).Scan(&inviteeWallets); err != nil {
		t.Fatal(err)
	}
	if inviterWallets != 0 || inviterLedgers != 0 || inviteeWallets != 0 {
		t.Fatalf("partial reward facts wallets=%d/%d inviterLedgers=%d", inviterWallets, inviteeWallets, inviterLedgers)
	}
}

func setRegistrationRewardConfig(t *testing.T, db *sql.DB, ctx context.Context, enabled bool, inviter, invitee int64) func() {
	t.Helper()
	var oldEnabled bool
	var oldInviter, oldInvitee int64
	var oldRemark string
	if err := db.QueryRowContext(ctx, `SELECT enabled,inviter_reward_coin,invitee_reward_coin,remark FROM reader_invite_reward_config WHERE id=1`).Scan(&oldEnabled, &oldInviter, &oldInvitee, &oldRemark); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE reader_invite_reward_config SET enabled=$1,inviter_reward_coin=$2,invitee_reward_coin=$3,remark='注册奖励集成测试' WHERE id=1`, enabled, inviter, invitee); err != nil {
		t.Fatal(err)
	}
	return func() {
		_, _ = db.ExecContext(context.Background(), `UPDATE reader_invite_reward_config SET enabled=$1,inviter_reward_coin=$2,invitee_reward_coin=$3,remark=$4 WHERE id=1`, oldEnabled, oldInviter, oldInvitee, oldRemark)
	}
}

func cleanupRegistrationRewardFacts(t *testing.T, db *sql.DB, readerIDs []int64) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = db.ExecContext(ctx, `ALTER TABLE reader_wallet_ledgers DISABLE TRIGGER reader_wallet_ledgers_immutable_update`)
		_, _ = db.ExecContext(ctx, `DELETE FROM reader_wallet_ledgers WHERE reader_id = ANY($1)`, readerIDs)
		_, _ = db.ExecContext(ctx, `ALTER TABLE reader_wallet_ledgers ENABLE TRIGGER reader_wallet_ledgers_immutable_update`)
		_, _ = db.ExecContext(ctx, `DELETE FROM reader_wallets WHERE reader_id = ANY($1)`, readerIDs)
		_, _ = db.ExecContext(ctx, `DELETE FROM reader_accounts WHERE id = ANY($1)`, readerIDs)
	})
}

func assertRegistrationRewardFacts(t *testing.T, db *sql.DB, ctx context.Context, inviterID, inviteeID, inviterBalance, inviteeBalance int64, ledgerCount int) {
	t.Helper()
	var gotInviter, gotInvitee int64
	var gotLedgers int
	if err := db.QueryRowContext(ctx, `SELECT bonus_coin_balance FROM reader_wallets WHERE reader_id=$1`, inviterID).Scan(&gotInviter); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT bonus_coin_balance FROM reader_wallets WHERE reader_id=$1`, inviteeID).Scan(&gotInvitee); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id IN ($1,$2) AND biz_type='invite_reward'`, inviterID, inviteeID).Scan(&gotLedgers); err != nil {
		t.Fatal(err)
	}
	if gotInviter != inviterBalance || gotInvitee != inviteeBalance || gotLedgers != ledgerCount {
		t.Fatalf("reward facts balances=%d/%d ledgers=%d", gotInviter, gotInvitee, gotLedgers)
	}
}
