package adminmembership

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	readercontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/google/uuid"
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
	in.RequestID = strings.TrimSpace(in.RequestID)
	in.ProductID = strings.TrimSpace(in.ProductID)
	in.Remark = strings.TrimSpace(in.Remark)
	in.DurationDays = strings.TrimSpace(in.DurationDays)
	if in.RequestID == "" {
		in.RequestID = uuid.NewString()
	}
	if readerID <= 0 || len(in.RequestID) > 64 || in.Remark == "" || len([]rune(in.Remark)) > 255 {
		return Grant{}, errors.New("invalid membership grant request")
	}
	if in.ProductID != "" {
		productID, err := strconv.ParseInt(in.ProductID, 10, 64)
		if err != nil || productID <= 0 {
			return Grant{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "会员商品ID必须是正整数字符串")
		}
		if s.Repo == nil {
			return Grant{}, apperror.New(apperror.CodeUnavailable, http.StatusServiceUnavailable, "读者服务暂不可用")
		}
		product, err := s.Repo.MembershipProduct(ctx, productID)
		if err != nil {
			return Grant{}, err
		}
		if product.DurationDays == nil {
			in.Permanent = true
			in.DurationDays = ""
		} else {
			in.Permanent = false
			in.DurationDays = strconv.Itoa(*product.DurationDays)
		}
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
