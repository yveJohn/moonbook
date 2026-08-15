package contract

import "context"

type RegistrationRewardRequest struct {
	RelationID int64
	InviterID  int64
	InviteeID  int64
}

type RegistrationRewardGranter interface {
	GrantRegistrationRewards(context.Context, RegistrationRewardRequest) error
}

type FirstRechargeRewardRequest struct {
	RelationID int64
	InviterID  int64
	InviteeID  int64
}

type FirstRechargeRewardGranter interface {
	GrantFirstRechargeReward(context.Context, FirstRechargeRewardRequest) error
}

type InviteRewardSummary struct {
	RegisterRewardCoin      int64
	FirstRechargeRewardCoin int64
	TotalRewardCoin         int64
}

type InviteRewardReader interface {
	InviteRewardSummary(context.Context, int64) (InviteRewardSummary, error)
}
