package admininvitereward

import (
	"context"
	"errors"
	"strconv"
)

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }

func valid(in Input) error {
	for _, v := range []string{in.InviterRewardCoin, in.InviteeRewardCoin} {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			return errors.New("invalid invite reward amount")
		}
	}
	if !in.Enabled && in.Remark == "" {
		return errors.New("invite reward remark is required")
	}
	if len([]rune(in.Remark)) > 255 {
		return errors.New("invite reward remark too long")
	}
	return nil
}

func (s *Service) Get(ctx context.Context) (Config, error) { return s.Repo.Get(ctx) }
func (s *Service) Update(ctx context.Context, in Input) (Config, error) {
	if err := valid(in); err != nil {
		return Config{}, err
	}
	return s.Repo.Update(ctx, in)
}
