package invitereward

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/wallet"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

const FirstRechargeRewardCoin int64 = 100

func GrantFirstRechargeTx(ctx context.Context, tx transaction.DBTX, inviteeID int64) error {
	key := fmt.Sprintf("invite_reward:%d:first_recharge", inviteeID)
	var rewardID, inviterID int64
	err := tx.QueryRowContext(ctx, `INSERT INTO reader_invite_reward_records(relation_id,inviter_reader_id,invitee_reader_id,reward_stage,reward_coin,status,idempotency_key,granted_at,remark) SELECT id,inviter_reader_id,invitee_reader_id,'first_recharge',$3,'granted',$2,now(),'邀请首充奖励' FROM reader_invite_relations WHERE invitee_reader_id=$1 AND status='active' ON CONFLICT(idempotency_key) DO NOTHING RETURNING id,inviter_reader_id`, inviteeID, key, FirstRechargeRewardCoin).Scan(&rewardID, &inviterID)
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
