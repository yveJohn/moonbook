package adminuser

import (
	"context"
	"errors"
	"testing"

	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
)

type serviceContextKey struct{}

type serviceTransactor struct{}

func (serviceTransactor) Within(ctx context.Context, fn func(context.Context) error) error {
	return fn(context.WithValue(ctx, serviceContextKey{}, true))
}

type serviceRepository struct {
	statusContexts []context.Context
	passwordCalls  int
}

func (*serviceRepository) List(context.Context, string, string, int, int) ([]User, int64, error) {
	return nil, 0, nil
}
func (*serviceRepository) Get(context.Context, int64) (User, error) { return User{}, nil }
func (repository *serviceRepository) SetStatus(ctx context.Context, id int64, status string) (User, error) {
	repository.statusContexts = append(repository.statusContexts, ctx)
	return User{ID: "21", Username: "reader-21", Nickname: "Moon", Status: status}, nil
}
func (repository *serviceRepository) ResetPassword(context.Context, int64, string, string) error {
	repository.passwordCalls++
	return nil
}

type serviceProjectionWriter struct {
	items []commercecontract.ReaderSearchProjection
	err   error
}

func (writer *serviceProjectionWriter) UpsertReaderSearchProjection(ctx context.Context, projection commercecontract.ReaderSearchProjection) error {
	if ctx.Value(serviceContextKey{}) != true {
		return errors.New("projection did not receive transaction context")
	}
	writer.items = append(writer.items, projection)
	return writer.err
}
func (*serviceProjectionWriter) DeleteReaderSearchProjection(context.Context, int64) error {
	return nil
}

func TestSetStatusSynchronizesProjectionInTransactionContext(t *testing.T) {
	repository := &serviceRepository{}
	projections := &serviceProjectionWriter{}
	service := NewService(repository, serviceTransactor{}, projections, nil)

	user, err := service.SetStatus(context.Background(), 21, "disabled")
	if err != nil {
		t.Fatal(err)
	}
	if len(repository.statusContexts) != 1 || repository.statusContexts[0].Value(serviceContextKey{}) != true {
		t.Fatal("repository did not receive transaction context")
	}
	want := commercecontract.ReaderSearchProjection{ReaderID: 21, Username: "reader-21", Nickname: "Moon", Status: "disabled"}
	if user.Status != "disabled" || len(projections.items) != 1 || projections.items[0] != want {
		t.Fatalf("user=%+v projections=%+v", user, projections.items)
	}
}

func TestSetStatusReturnsProjectionFailure(t *testing.T) {
	want := errors.New("projection unavailable")
	projections := &serviceProjectionWriter{err: want}
	service := NewService(&serviceRepository{}, serviceTransactor{}, projections, nil)

	if _, err := service.SetStatus(context.Background(), 21, "disabled"); !errors.Is(err, want) {
		t.Fatalf("error=%v", err)
	}
}

func TestResetPasswordDoesNotSynchronizeProjection(t *testing.T) {
	repository := &serviceRepository{}
	projections := &serviceProjectionWriter{}
	service := NewService(repository, serviceTransactor{}, projections, nil)

	if err := service.ResetPassword(context.Background(), 21, "new-pass", "new-pass"); err != nil {
		t.Fatal(err)
	}
	if repository.passwordCalls != 1 || len(projections.items) != 0 {
		t.Fatalf("passwordCalls=%d projections=%+v", repository.passwordCalls, projections.items)
	}
}
