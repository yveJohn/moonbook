package admininvitereward

import "context"

type Config struct {
	ID, Enabled, InviterRewardCoin, InviteeRewardCoin, Remark, UpdatedAt string
}

type Input struct {
	Enabled           bool   `json:"enabled"`
	InviterRewardCoin string `json:"inviterRewardCoin"`
	InviteeRewardCoin string `json:"inviteeRewardCoin"`
	Remark            string `json:"remark"`
}

type Repository interface {
	Get(context.Context) (Config, error)
	Update(context.Context, Input) (Config, error)
}
