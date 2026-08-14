package adminmembership

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }

func (s *Service) Grant(ctx context.Context, readerID int64, in Input) (Grant, error) {
	if readerID <= 0 || strings.TrimSpace(in.RequestID) == "" || len(in.RequestID) > 64 || strings.TrimSpace(in.Remark) == "" || len([]rune(in.Remark)) > 255 {
		return Grant{}, errors.New("invalid membership grant request")
	}
	if in.Permanent {
		if in.DurationDays != "" {
			return Grant{}, errors.New("permanent membership cannot set duration")
		}
	} else {
		days, err := strconv.Atoi(in.DurationDays)
		if err != nil || days <= 0 || days > 36500 {
			return Grant{}, errors.New("invalid membership duration")
		}
	}
	return s.Repo.Grant(ctx, readerID, in)
}
