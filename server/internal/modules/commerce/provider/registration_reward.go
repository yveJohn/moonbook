package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

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
	businessID := fmt.Sprintf("%d:%d", request.RelationID, request.InviteeID)
	if _, err := executor.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "commerce-registration-reward:"+businessID); err != nil {
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
		key := businessID + ":inviter"
		remark := "邀请奖励"
		mutations = append(mutations, wallet.Mutation{ReaderID: request.InviterID, BizType: "invite_reward", BizID: &businessID, Direction: "income", CoinType: "bonus", Amount: inviterReward, LedgerNo: "INVI-" + businessID, Remark: &remark, IdempotencyKey: &key})
	}
	if inviteeReward > 0 {
		key := businessID + ":invitee"
		remark := "注册奖励"
		mutations = append(mutations, wallet.Mutation{ReaderID: request.InviteeID, BizType: "invite_reward", BizID: &businessID, Direction: "income", CoinType: "bonus", Amount: inviteeReward, LedgerNo: "INVE-" + businessID, Remark: &remark, IdempotencyKey: &key})
	}
	sort.Slice(mutations, func(left, right int) bool { return mutations[left].ReaderID < mutations[right].ReaderID })
	for _, mutation := range mutations {
		if _, err := wallet.MutateTx(ctx, executor, mutation); err != nil {
			return contract.Wrap(contract.ErrUnavailable, err)
		}
	}
	return nil
}
