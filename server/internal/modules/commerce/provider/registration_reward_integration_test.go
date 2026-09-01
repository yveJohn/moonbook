//go:build integration

package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
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
	inviteeID, inviterID, relationID := base, base+1, base+2
	prefix := fmt.Sprintf("registration-reward-it-%d", base)
	restoreRewardConfig := setRegistrationRewardConfig(t, db, ctx, true, 13, 7)
	t.Cleanup(restoreRewardConfig)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'fixture','enabled'),($3,$4,'fixture','enabled')`, inviterID, prefix+"-inviter", inviteeID, prefix+"-invitee"); err != nil {
		t.Fatal(err)
	}
	insertRegistrationRewardRelation(t, db, ctx, relationID, inviterID, inviteeID, prefix)
	cleanupRegistrationRewardFacts(t, db, []int64{inviterID, inviteeID})

	provider := NewRegistrationReward(db)
	transactor := transaction.New(db)
	request := commercecontract.RegistrationRewardRequest{RelationID: relationID, InviterID: inviterID, InviteeID: inviteeID}
	const workers = 16
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
	assertRegistrationRewardFacts(t, db, ctx, relationID, inviterID, inviteeID, 13, 7)
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
	insertRegistrationRewardRelation(t, db, ctx, relationID, inviterID, inviteeID, prefix)
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
	var inviterWallets, inviterLedgers, inviteeWallets, rewardFacts int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallets WHERE reader_id=$1`, inviterID).Scan(&inviterWallets); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1`, inviterID).Scan(&inviterLedgers); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallets WHERE reader_id=$1`, inviteeID).Scan(&inviteeWallets); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_invite_reward_records WHERE invitee_reader_id=$1`, inviteeID).Scan(&rewardFacts); err != nil {
		t.Fatal(err)
	}
	if inviterWallets != 0 || inviterLedgers != 0 || inviteeWallets != 0 || rewardFacts != 0 {
		t.Fatalf("partial reward facts wallets=%d/%d inviterLedgers=%d rewardFacts=%d", inviterWallets, inviteeWallets, inviterLedgers, rewardFacts)
	}
}

func TestRegistrationRewardConfigurationModes(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	inviteeID, inviterID, relationID := base, base+1, base+2
	prefix := fmt.Sprintf("registration-reward-zero-%d", base)
	restoreRewardConfig := setRegistrationRewardConfig(t, db, ctx, true, 0, 7)
	t.Cleanup(restoreRewardConfig)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'fixture','enabled'),($3,$4,'fixture','enabled')`, inviterID, prefix+"-inviter", inviteeID, prefix+"-invitee"); err != nil {
		t.Fatal(err)
	}
	insertRegistrationRewardRelation(t, db, ctx, relationID, inviterID, inviteeID, prefix)
	cleanupRegistrationRewardFacts(t, db, []int64{inviterID, inviteeID})

	err := transaction.New(db).Within(ctx, func(txCtx context.Context) error {
		return NewRegistrationReward(db).GrantRegistrationRewards(txCtx, commercecontract.RegistrationRewardRequest{RelationID: relationID, InviterID: inviterID, InviteeID: inviteeID})
	})
	if err != nil {
		t.Fatal(err)
	}
	var rewardFacts, inviterLedgers, inviteeLedgers int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_invite_reward_records WHERE invitee_reader_id=$1`, inviteeID).Scan(&rewardFacts); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1`, inviterID).Scan(&inviterLedgers); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1 AND biz_type='invite_reward'`, inviteeID).Scan(&inviteeLedgers); err != nil {
		t.Fatal(err)
	}
	if rewardFacts != 0 || inviterLedgers != 0 || inviteeLedgers != 1 {
		t.Fatalf("zero inviter reward facts=%d inviterLedgers=%d inviteeLedgers=%d", rewardFacts, inviterLedgers, inviteeLedgers)
	}

	disabledInviterID, disabledInviteeID, disabledRelationID := base+10, base+11, base+12
	disabledPrefix := prefix + "-disabled"
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'fixture','enabled'),($3,$4,'fixture','enabled')`, disabledInviterID, disabledPrefix+"-inviter", disabledInviteeID, disabledPrefix+"-invitee"); err != nil {
		t.Fatal(err)
	}
	insertRegistrationRewardRelation(t, db, ctx, disabledRelationID, disabledInviterID, disabledInviteeID, disabledPrefix)
	cleanupRegistrationRewardFacts(t, db, []int64{disabledInviterID, disabledInviteeID})
	if _, err := db.ExecContext(ctx, `UPDATE reader_invite_reward_config SET enabled=false,inviter_reward_coin=13,invitee_reward_coin=7 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	err = transaction.New(db).Within(ctx, func(txCtx context.Context) error {
		return NewRegistrationReward(db).GrantRegistrationRewards(txCtx, commercecontract.RegistrationRewardRequest{RelationID: disabledRelationID, InviterID: disabledInviterID, InviteeID: disabledInviteeID})
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_invite_reward_records WHERE invitee_reader_id=$1`, disabledInviteeID).Scan(&rewardFacts); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id IN ($1,$2)`, disabledInviterID, disabledInviteeID).Scan(&inviterLedgers); err != nil {
		t.Fatal(err)
	}
	if rewardFacts != 0 || inviterLedgers != 0 {
		t.Fatalf("disabled reward facts=%d ledgers=%d", rewardFacts, inviterLedgers)
	}
}

func TestRegistrationRewardRejectsConflictingFact(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	inviterID, inviteeID, relationID := base, base+1, base+2
	prefix := fmt.Sprintf("registration-reward-conflict-%d", base)
	restoreRewardConfig := setRegistrationRewardConfig(t, db, ctx, true, 13, 7)
	t.Cleanup(restoreRewardConfig)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'fixture','enabled'),($3,$4,'fixture','enabled')`, inviterID, prefix+"-inviter", inviteeID, prefix+"-invitee"); err != nil {
		t.Fatal(err)
	}
	insertRegistrationRewardRelation(t, db, ctx, relationID, inviterID, inviteeID, prefix)
	cleanupRegistrationRewardFacts(t, db, []int64{inviterID, inviteeID})
	key := fmt.Sprintf("invite_reward:%d:register", inviteeID)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_reward_records(relation_id,inviter_reader_id,invitee_reader_id,reward_stage,reward_coin,status,idempotency_key,granted_at) VALUES($1,$2,$3,'register',99,'granted',$4,now())`, relationID, inviterID, inviteeID, key); err != nil {
		t.Fatal(err)
	}

	err := transaction.New(db).Within(ctx, func(txCtx context.Context) error {
		return NewRegistrationReward(db).GrantRegistrationRewards(txCtx, commercecontract.RegistrationRewardRequest{RelationID: relationID, InviterID: inviterID, InviteeID: inviteeID})
	})
	if !errors.Is(err, commercecontract.ErrIdempotencyConflict) {
		t.Fatalf("conflicting fact error=%v", err)
	}
	var ledgers, wallets int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id IN ($1,$2)`, inviterID, inviteeID).Scan(&ledgers); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallets WHERE reader_id IN ($1,$2)`, inviterID, inviteeID).Scan(&wallets); err != nil {
		t.Fatal(err)
	}
	if ledgers != 0 || wallets != 0 {
		t.Fatalf("conflicting fact left ledgers=%d wallets=%d", ledgers, wallets)
	}
}

func TestRegistrationRewardRollsBackWhenInviterLedgerFails(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	inviterID, inviteeID, blockerID, relationID := base, base+1, base+2, base+3
	prefix := fmt.Sprintf("registration-reward-inviter-failure-%d", base)
	restoreRewardConfig := setRegistrationRewardConfig(t, db, ctx, true, 13, 7)
	t.Cleanup(restoreRewardConfig)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'fixture','enabled'),($3,$4,'fixture','enabled'),($5,$6,'fixture','enabled')`, inviterID, prefix+"-inviter", inviteeID, prefix+"-invitee", blockerID, prefix+"-blocker"); err != nil {
		t.Fatal(err)
	}
	insertRegistrationRewardRelation(t, db, ctx, relationID, inviterID, inviteeID, prefix)
	cleanupRegistrationRewardFacts(t, db, []int64{inviterID, inviteeID, blockerID})
	key := fmt.Sprintf("invite_reward:%d:register", inviteeID)
	var rewardID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_invite_reward_records(relation_id,inviter_reader_id,invitee_reader_id,reward_stage,reward_coin,status,idempotency_key,granted_at,remark) VALUES($1,$2,$3,'register',13,'granted',$4,now(),'邀请注册奖励') RETURNING id`, relationID, inviterID, inviteeID, key).Scan(&rewardID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_wallets(reader_id) VALUES($1)`, blockerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_wallet_ledgers(reader_id,ledger_no,biz_type,direction,coin_type,amount,balance_before,balance_after) VALUES($1,$2,'fixture','income','bonus',1,0,1)`, blockerID, "INVR-"+strconv.FormatInt(rewardID, 10)); err != nil {
		t.Fatal(err)
	}

	err := transaction.New(db).Within(ctx, func(txCtx context.Context) error {
		return NewRegistrationReward(db).GrantRegistrationRewards(txCtx, commercecontract.RegistrationRewardRequest{RelationID: relationID, InviterID: inviterID, InviteeID: inviteeID})
	})
	if err == nil {
		t.Fatal("expected inviter ledger conflict")
	}
	var rewardWallets, rewardLedgers int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallets WHERE reader_id IN ($1,$2)`, inviterID, inviteeID).Scan(&rewardWallets); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id IN ($1,$2)`, inviterID, inviteeID).Scan(&rewardLedgers); err != nil {
		t.Fatal(err)
	}
	if rewardWallets != 0 || rewardLedgers != 0 {
		t.Fatalf("inviter failure left reward wallets=%d ledgers=%d", rewardWallets, rewardLedgers)
	}
}

func insertRegistrationRewardRelation(t *testing.T, db *sql.DB, ctx context.Context, relationID, inviterID, inviteeID int64, prefix string) {
	t.Helper()
	codeID := relationID + 1
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status) VALUES($1,$2,$3,'enabled')`, codeID, prefix+"-code", inviterID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_relations(id,inviter_reader_id,invitee_reader_id,invite_code_id,status) VALUES($1,$2,$3,$4,'active')`, relationID, inviterID, inviteeID, codeID); err != nil {
		t.Fatal(err)
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
		_, _ = db.ExecContext(ctx, `DELETE FROM reader_invite_reward_records WHERE inviter_reader_id = ANY($1) OR invitee_reader_id = ANY($1)`, readerIDs)
		_, _ = db.ExecContext(ctx, `ALTER TABLE reader_wallet_ledgers DISABLE TRIGGER reader_wallet_ledgers_immutable_update`)
		_, _ = db.ExecContext(ctx, `DELETE FROM reader_wallet_ledgers WHERE reader_id = ANY($1)`, readerIDs)
		_, _ = db.ExecContext(ctx, `ALTER TABLE reader_wallet_ledgers ENABLE TRIGGER reader_wallet_ledgers_immutable_update`)
		_, _ = db.ExecContext(ctx, `DELETE FROM reader_wallets WHERE reader_id = ANY($1)`, readerIDs)
		_, _ = db.ExecContext(ctx, `DELETE FROM reader_invite_relations WHERE inviter_reader_id = ANY($1) OR invitee_reader_id = ANY($1)`, readerIDs)
		_, _ = db.ExecContext(ctx, `DELETE FROM reader_invite_codes WHERE inviter_reader_id = ANY($1)`, readerIDs)
		_, _ = db.ExecContext(ctx, `DELETE FROM reader_accounts WHERE id = ANY($1)`, readerIDs)
	})
}

func assertRegistrationRewardFacts(t *testing.T, db *sql.DB, ctx context.Context, relationID, inviterID, inviteeID, inviterBalance, inviteeBalance int64) {
	t.Helper()
	var gotInviter, gotInvitee int64
	var inviterLedgers, inviteeLedgers int
	if err := db.QueryRowContext(ctx, `SELECT bonus_coin_balance FROM reader_wallets WHERE reader_id=$1`, inviterID).Scan(&gotInviter); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT bonus_coin_balance FROM reader_wallets WHERE reader_id=$1`, inviteeID).Scan(&gotInvitee); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1 AND biz_type='invite_register_reward'`, inviterID).Scan(&inviterLedgers); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1 AND biz_type='invite_reward'`, inviteeID).Scan(&inviteeLedgers); err != nil {
		t.Fatal(err)
	}
	var factRelation, factInviter, factInvitee, factCoin int64
	var factStage, factStatus, factKey, factRemark, ledgerBizID string
	if err := db.QueryRowContext(ctx, `SELECT relation_id,inviter_reader_id,invitee_reader_id,reward_stage,reward_coin,status,idempotency_key,remark FROM reader_invite_reward_records WHERE invitee_reader_id=$1 AND reward_stage='register'`, inviteeID).Scan(&factRelation, &factInviter, &factInvitee, &factStage, &factCoin, &factStatus, &factKey, &factRemark); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT biz_id FROM reader_wallet_ledgers WHERE reader_id=$1 AND biz_type='invite_register_reward'`, inviterID).Scan(&ledgerBizID); err != nil {
		t.Fatal(err)
	}
	var rewardIDText string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM reader_invite_reward_records WHERE invitee_reader_id=$1 AND reward_stage='register'`, inviteeID).Scan(&rewardIDText); err != nil {
		t.Fatal(err)
	}
	wantKey := fmt.Sprintf("invite_reward:%d:register", inviteeID)
	if gotInviter != inviterBalance || gotInvitee != inviteeBalance || inviterLedgers != 1 || inviteeLedgers != 1 || factRelation != relationID || factInviter != inviterID || factInvitee != inviteeID || factStage != "register" || factCoin != inviterBalance || factStatus != "granted" || factKey != wantKey || factRemark != "邀请注册奖励" || ledgerBizID != rewardIDText {
		t.Fatalf("reward facts balances=%d/%d ledgers=%d/%d fact=%d/%d/%d/%s/%d/%s/%s/%s ledgerBizID=%s rewardID=%s", gotInviter, gotInvitee, inviterLedgers, inviteeLedgers, factRelation, factInviter, factInvitee, factStage, factCoin, factStatus, factKey, factRemark, ledgerBizID, rewardIDText)
	}
}
