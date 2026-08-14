package checkin

import "context"

type Status struct {
	TodayChecked                  bool
	ContinuousDays                int
	TodayRewardCoin               int64
	RewardRandom                  bool
	RewardText, UnavailableReason string
	CheckinAvailable              bool
}
type Repository interface {
	Status(context.Context, int64) (Status, error)
	Checkin(context.Context, int64) (Status, error)
}
