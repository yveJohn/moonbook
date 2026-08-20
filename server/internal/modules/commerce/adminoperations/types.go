package adminoperations

import "context"

type Checkin struct{ ID, ReaderID, Username, Nickname, CheckinDate, ContinuousDays, BaseRewardCoin, MilestoneRewardCoin, TotalRewardCoin, IdempotencyKey, CreatedAt string }
type InviteReward struct{ ID, RelationID, InviterReaderID, InviterUsername, InviterNickname, InviteeReaderID, InviteeUsername, InviteeNickname, RewardStage, RewardCoin, Status, IdempotencyKey, GrantedAt, Remark, CreatedAt, UpdatedAt string }
type CheckinFilter struct {
	ReaderKeyword, StartDate, EndDate string
	Page, PageSize                    int
}
type InviteRewardFilter struct {
	InviterKeyword, InviteeKeyword, RewardStage, Status, StartTime, EndTime string
	Page, PageSize                                                          int
}
type Repository interface {
	ListCheckins(context.Context, CheckinFilter) ([]Checkin, int64, error)
	ListInviteRewards(context.Context, InviteRewardFilter) ([]InviteReward, int64, error)
}
type Service struct{ Repo Repository }

func NewService(r Repository) *Service { return &Service{Repo: r} }
func (s *Service) ListCheckins(ctx context.Context, f CheckinFilter) ([]Checkin, int64, error) {
	return s.Repo.ListCheckins(ctx, f)
}
func (s *Service) ListInviteRewards(ctx context.Context, f InviteRewardFilter) ([]InviteReward, int64, error) {
	return s.Repo.ListInviteRewards(ctx, f)
}
