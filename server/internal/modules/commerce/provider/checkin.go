package provider

import (
	"context"
	"errors"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/checkin"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
)

type Checkin struct{ service *checkin.Service }

var _ contract.CheckinReader = (*Checkin)(nil)

func NewCheckin(service *checkin.Service) *Checkin { return &Checkin{service: service} }

func (provider *Checkin) CheckinStatus(ctx context.Context, readerID int64) (contract.CheckinStatus, error) {
	if provider == nil || provider.service == nil {
		return contract.CheckinStatus{}, contract.ErrUnavailable
	}
	status, err := provider.service.Status(ctx, readerID)
	return checkinStatus(status), checkinError(err)
}

func (provider *Checkin) Checkin(ctx context.Context, readerID int64) (contract.CheckinStatus, error) {
	if provider == nil || provider.service == nil {
		return contract.CheckinStatus{}, contract.ErrUnavailable
	}
	status, err := provider.service.Checkin(ctx, readerID)
	return checkinStatus(status), checkinError(err)
}

func checkinStatus(status checkin.Status) contract.CheckinStatus {
	return contract.CheckinStatus{
		TodayChecked: status.TodayChecked, ContinuousDays: status.ContinuousDays,
		TodayRewardCoin: status.TodayRewardCoin, RewardRandom: status.RewardRandom,
		RewardText: status.RewardText, UnavailableReason: status.UnavailableReason,
		CheckinAvailable: status.CheckinAvailable,
	}
}

func checkinError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, checkin.ErrUnavailable) {
		return contract.Wrap(contract.ErrProductUnavailable, err)
	}
	return contract.Wrap(contract.ErrUnavailable, err)
}
