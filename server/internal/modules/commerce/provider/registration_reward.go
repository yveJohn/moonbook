package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
	businessID := fmt.Sprintf("%d:%d", request.RelationID, request.InviteeID)
	if inviterReward > 0 {
		key := businessID + ":inviter"
		remark := "邀请奖励"
		if _, err := wallet.MutateTx(ctx, executor, wallet.Mutation{ReaderID: request.InviterID, BizType: "invite_reward", BizID: &businessID, Direction: "income", CoinType: "bonus", Amount: inviterReward, LedgerNo: "INVI-" + businessID, Remark: &remark, IdempotencyKey: &key}); err != nil {
			return contract.Wrap(contract.ErrUnavailable, err)
		}
	}
	if inviteeReward > 0 {
		key := businessID + ":invitee"
		remark := "注册奖励"
		if _, err := wallet.MutateTx(ctx, executor, wallet.Mutation{ReaderID: request.InviteeID, BizType: "invite_reward", BizID: &businessID, Direction: "income", CoinType: "bonus", Amount: inviteeReward, LedgerNo: "INVE-" + businessID, Remark: &remark, IdempotencyKey: &key}); err != nil {
			return contract.Wrap(contract.ErrUnavailable, err)
		}
	}
	return nil
}
