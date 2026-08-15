package adminproduct

import (
	"context"
	"errors"
	"testing"

	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

type targetStub struct {
	snapshot novelcontract.TargetSnapshot
	err      error
}

func (stub targetStub) ProductTarget(context.Context, string, int64) (novelcontract.TargetSnapshot, error) {
	return stub.snapshot, stub.err
}

type productRepositoryStub struct{ created bool }

func (stub *productRepositoryStub) List(context.Context, string, string, string, int, int) ([]Product, int64, error) {
	return nil, 0, nil
}
func (stub *productRepositoryStub) Create(_ context.Context, input Input) (Product, error) {
	stub.created = true
	return Product{ProductType: input.ProductType, TargetID: input.TargetID}, nil
}
func (stub *productRepositoryStub) Update(context.Context, int64, Input) (Product, error) {
	return Product{}, nil
}
func (stub *productRepositoryStub) Delete(context.Context, int64) error { return nil }

func TestServiceRejectsUnavailableNovelTargetsBeforeWriting(t *testing.T) {
	repository := &productRepositoryStub{}
	input := Input{ProductType: "book", TargetID: "7", ProductName: "作品", PriceCoin: "1", SaleStatus: "on_sale"}
	service := NewService(repository, targetStub{snapshot: novelcontract.TargetSnapshot{Type: "book", TargetID: 7, Enabled: false}})
	if _, err := service.Create(context.Background(), input); err == nil || apperror.Expose(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("disabled target err=%v", err)
	}
	if repository.created {
		t.Fatal("repository must not be called for an unavailable target")
	}
	service = NewService(repository, targetStub{err: novelcontract.ErrUnavailable})
	if _, err := service.Create(context.Background(), input); err == nil || apperror.Expose(err).Code != apperror.CodeUnavailable || !errors.Is(err, novelcontract.ErrUnavailable) {
		t.Fatalf("provider failure err=%v", err)
	}
}
