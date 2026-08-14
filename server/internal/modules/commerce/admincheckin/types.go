package admincheckin

import "context"

type Rule struct{ ID, RuleType, ContinuousDays, RewardMode, FixedCoin, MinCoin, MaxCoin, Status, SortOrder, Remark string }
type Input struct{ RuleType, ContinuousDays, RewardMode, FixedCoin, MinCoin, MaxCoin, Status, SortOrder, Remark string }
type Repository interface {
	List(context.Context, string, int, int) ([]Rule, int64, error)
	Create(context.Context, Input) (Rule, error)
	Update(context.Context, int64, Input) (Rule, error)
	Delete(context.Context, int64) error
}
