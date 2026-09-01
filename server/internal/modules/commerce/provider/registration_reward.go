package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/wallet"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

type RegistrationReward struct {
	db *sql.DB
}

var _ contract.RegistrationRewardGranter = (*RegistrationReward)(nil)

func NewRegistrationReward(db *sql.DB) *RegistrationReward {
	return &RegistrationReward{db: db}
}

func (provider *RegistrationReward) GrantRegistrationRewards(ctx context.Context, request contract.RegistrationRewardRequest) error {
	if provider == nil || provider.db == nil || request.RelationID <= 0 || request.InviterID <= 0 || request.InviteeID <= 0 || request.InviterID == request.InviteeID {
		return contract.Wrap(contract.ErrInvalidRequest, errors.New("invalid registration reward request"))
	}
	executor := transaction.Executor(ctx, provider.db)
	if executor == provider.db {
		return contract.Wrap(contract.ErrUnavailable, transaction.ErrNoTransaction)
	}
	legacyBusinessID := fmt.Sprintf("%d:%d", request.RelationID, request.InviteeID)
	if _, err := executor.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "commerce-registration-reward:"+legacyBusinessID); err != nil {
		return contract.Wrap(contract.ErrUnavailable, err)
	}
	var enabled bool
	var inviterReward, inviteeReward int64
	if err := executor.QueryRowContext(ctx, `
SELECT enabled, inviter_reward_coin, invitee_reward_coin
FROM reader_invite_reward_config
WHERE id=1`).Scan(&enabled, &inviterReward, &inviteeReward); err != nil {
		return contract.Wrap(contract.ErrUnavailable, err)
	}
	if !enabled {
		return nil
	}
	mutations := make([]wallet.Mutation, 0, 2)
	if inviterReward > 0 {
		rewardID, key, err := registrationRewardFact(ctx, executor, request, inviterReward)
		if err != nil {
			return err
		}
		rewardIDText := strconv.FormatInt(rewardID, 10)
		remark := "邀请注册奖励"
		mutations = append(mutations, wallet.Mutation{ReaderID: request.InviterID, BizType: "invite_register_reward", BizID: &rewardIDText, Direction: "income", CoinType: "bonus", Amount: inviterReward, LedgerNo: "INVR-" + rewardIDText, Remark: &remark, IdempotencyKey: &key})
	}
	if inviteeReward > 0 {
		key := legacyBusinessID + ":invitee"
		remark := "注册奖励"
		mutations = append(mutations, wallet.Mutation{ReaderID: request.InviteeID, BizType: "invite_reward", BizID: &legacyBusinessID, Direction: "income", CoinType: "bonus", Amount: inviteeReward, LedgerNo: "INVE-" + legacyBusinessID, Remark: &remark, IdempotencyKey: &key})
	}
	sort.Slice(mutations, func(left, right int) bool { return mutations[left].ReaderID < mutations[right].ReaderID })
	for _, mutation := range mutations {
		ledger, err := wallet.MutateTx(ctx, executor, mutation)
		if err != nil {
			return contract.Wrap(contract.ErrUnavailable, err)
		}
		if !registrationLedgerMatches(ledger, mutation) {
			return contract.Wrap(contract.ErrIdempotencyConflict, errors.New("registration reward ledger does not match request"))
		}
	}
	return nil
}

func registrationLedgerMatches(ledger wallet.Ledger, mutation wallet.Mutation) bool {
	return ledger.ReaderID == mutation.ReaderID && ledger.LedgerNo == mutation.LedgerNo && ledger.BizType == mutation.BizType &&
		ledger.Direction == mutation.Direction && ledger.CoinType == mutation.CoinType && ledger.Amount == mutation.Amount &&
		equalRegistrationRewardText(ledger.BizID, mutation.BizID) && equalRegistrationRewardText(ledger.IdempotencyKey, mutation.IdempotencyKey)
}

func equalRegistrationRewardText(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func registrationRewardFact(ctx context.Context, executor transaction.DBTX, request contract.RegistrationRewardRequest, rewardCoin int64) (int64, string, error) {
	key := fmt.Sprintf("invite_reward:%d:register", request.InviteeID)
	var rewardID int64
	err := executor.QueryRowContext(ctx, `
INSERT INTO reader_invite_reward_records(
    relation_id,inviter_reader_id,invitee_reader_id,reward_stage,reward_coin,status,idempotency_key,granted_at,remark
) VALUES($1,$2,$3,'register',$4,'granted',$5,now(),'邀请注册奖励')
ON CONFLICT DO NOTHING
RETURNING id`, request.RelationID, request.InviterID, request.InviteeID, rewardCoin, key).Scan(&rewardID)
	if err == nil {
		return rewardID, key, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, "", contract.Wrap(contract.ErrUnavailable, err)
	}

	var relationID, inviterID, inviteeID, existingCoin int64
	var stage, status, existingKey string
	err = executor.QueryRowContext(ctx, `
SELECT id,relation_id,inviter_reader_id,invitee_reader_id,reward_stage,reward_coin,status,idempotency_key
FROM reader_invite_reward_records
WHERE invitee_reader_id=$1 AND reward_stage='register'`, request.InviteeID).Scan(
		&rewardID, &relationID, &inviterID, &inviteeID, &stage, &existingCoin, &status, &existingKey,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", contract.Wrap(contract.ErrIdempotencyConflict, errors.New("registration reward key is already used"))
	}
	if err != nil {
		return 0, "", contract.Wrap(contract.ErrUnavailable, err)
	}
	if relationID != request.RelationID || inviterID != request.InviterID || inviteeID != request.InviteeID || stage != "register" || existingCoin != rewardCoin || status != "granted" || existingKey != key {
		return 0, "", contract.Wrap(contract.ErrIdempotencyConflict, errors.New("registration reward fact does not match request"))
	}
	return rewardID, key, nil
}
