package adminproduct

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

type Service struct {
	Repo    Repository
	Targets novelcontract.ProductTargetReader
}

func NewService(repo Repository, targets novelcontract.ProductTargetReader) *Service {
	return &Service{Repo: repo, Targets: targets}
}
func (s *Service) List(ctx context.Context, keyword, productType, status string, page, size int) ([]Product, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return s.Repo.List(ctx, keyword, productType, status, page, size)
}
func (s *Service) Create(ctx context.Context, in Input) (Product, error) {
	var err error
	in, err = normalize(in)
	if err != nil {
		return Product{}, err
	}
	if err = s.validateTarget(ctx, in); err != nil {
		return Product{}, err
	}
	return s.Repo.Create(ctx, in)
}
func (s *Service) Update(ctx context.Context, id int64, in Input) (Product, error) {
	var err error
	in, err = normalize(in)
	if err != nil {
		return Product{}, err
	}
	if err = s.validateTarget(ctx, in); err != nil {
		return Product{}, err
	}
	return s.Repo.Update(ctx, id, in)
}
func (s *Service) Delete(ctx context.Context, id int64) error { return s.Repo.Delete(ctx, id) }

func (s *Service) validateTarget(ctx context.Context, in Input) error {
	if in.ProductType != "book" && in.ProductType != "chapter" {
		return nil
	}
	if s.Targets == nil {
		return apperror.New(apperror.CodeUnavailable, http.StatusServiceUnavailable, "小说服务暂不可用")
	}
	targetID, _ := strconv.ParseInt(in.TargetID, 10, 64)
	snapshot, err := s.Targets.ProductTarget(ctx, in.ProductType, targetID)
	if err != nil {
		return productTargetError(err)
	}
	if !snapshot.Enabled || snapshot.Type != in.ProductType || snapshot.TargetID != targetID {
		return apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "商品目标不可用")
	}
	return nil
}

func productTargetError(err error) error {
	if errors.Is(err, novelcontract.ErrBookNotFound) || errors.Is(err, novelcontract.ErrChapterNotFound) || errors.Is(err, novelcontract.ErrNotPublished) || errors.Is(err, novelcontract.ErrTargetUnavailable) {
		return apperror.Wrap(err, apperror.CodeInvalidArgument, http.StatusBadRequest, "商品目标不可用")
	}
	return apperror.Wrap(err, apperror.CodeUnavailable, http.StatusServiceUnavailable, "小说服务暂不可用")
}
