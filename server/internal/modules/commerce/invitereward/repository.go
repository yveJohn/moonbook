package invitereward

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/wallet"
	readercontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

const FirstRechargeRewardCoin int64 = 100

func GrantFirstRechargeTx(ctx context.Context, tx transaction.DBTX, invites readercontract.InviteRelationReader, inviteeID int64) error {
	if invites == nil {
		return readercontract.ErrUnavailable
	}
	relation, err := invites.ActiveInviteRelation(ctx, inviteeID)
	if errors.Is(err, readercontract.ErrInviteRelationNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if relation.ID <= 0 || relation.InviterID <= 0 || relation.InviteeID != inviteeID || relation.Status != "active" {
		return readercontract.ErrUnavailable
	}
	key := fmt.Sprintf("invite_reward:%d:first_recharge", inviteeID)
	var rewardID, inviterID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO reader_invite_reward_records(relation_id,inviter_reader_id,invitee_reader_id,reward_stage,reward_coin,status,idempotency_key,granted_at,remark) VALUES($1,$2,$3,'first_recharge',$4,'granted',$5,now(),'邀请首充奖励') ON CONFLICT(idempotency_key) DO NOTHING RETURNING id,inviter_reader_id`, relation.ID, relation.InviterID, inviteeID, FirstRechargeRewardCoin, key).Scan(&rewardID, &inviterID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	rewardIDText := strconv.FormatInt(rewardID, 10)
	remark := "邀请首充奖励"
	_, err = wallet.MutateTx(ctx, tx, wallet.Mutation{ReaderID: inviterID, LedgerNo: "IFR-" + rewardIDText, BizType: "invite_first_recharge_reward", BizID: &rewardIDText, Direction: "income", CoinType: "bonus", Amount: FirstRechargeRewardCoin, Remark: &remark, IdempotencyKey: &key})
	return err
}
