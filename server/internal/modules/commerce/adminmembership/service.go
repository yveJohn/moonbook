package adminmembership

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	readercontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

type Service struct {
	Repo   Repository
	Tx     Transactor
	Reader readercontract.AccountLocker
}

func NewService(repo Repository, tx Transactor, reader readercontract.AccountLocker) *Service {
	return &Service{Repo: repo, Tx: tx, Reader: reader}
}

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
	if s.Repo == nil || s.Tx == nil || s.Reader == nil {
		return Grant{}, apperror.New(apperror.CodeUnavailable, http.StatusServiceUnavailable, "读者服务暂不可用")
	}
	var grant Grant
	err := s.Tx.Within(ctx, func(txCtx context.Context) error {
		if _, err := s.Reader.LockAccount(txCtx, readerID); err != nil {
			return err
		}
		var err error
		grant, err = s.Repo.Grant(txCtx, readerID, in)
		return err
	})
	if err != nil {
		return Grant{}, readerError(err)
	}
	return grant, nil
}

func readerError(err error) error {
	if errors.Is(err, readercontract.ErrAccountNotFound) {
		return apperror.Wrap(err, apperror.CodeNotFound, http.StatusNotFound, "读者不存在")
	}
	if errors.Is(err, readercontract.ErrAccountDisabled) {
		return apperror.Wrap(err, apperror.CodeConflict, http.StatusConflict, "读者账号已禁用")
	}
	if errors.Is(err, readercontract.ErrUnavailable) {
		return apperror.Wrap(err, apperror.CodeUnavailable, http.StatusServiceUnavailable, "读者服务暂不可用")
	}
	return err
}
