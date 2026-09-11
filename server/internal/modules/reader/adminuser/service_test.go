package adminuser

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	users          []User
	operations     []Operation
	operationsErr  error
	getErr         error
}

func (repository *serviceRepository) List(context.Context, string, string, int, int) ([]User, int64, error) {
	return repository.users, int64(len(repository.users)), nil
}
func (repository *serviceRepository) Get(_ context.Context, id int64) (User, error) {
	if repository.getErr != nil {
		return User{}, repository.getErr
	}
	return User{ID: strconv.FormatInt(id, 10)}, nil
}
func (repository *serviceRepository) SetStatus(ctx context.Context, id int64, status string) (User, error) {
	repository.statusContexts = append(repository.statusContexts, ctx)
	return User{ID: "21", Username: "reader-21", Nickname: "Moon", Status: status}, nil
}
func (repository *serviceRepository) ResetPassword(context.Context, int64, string, string) error {
	repository.passwordCalls++
	return nil
}
func (repository *serviceRepository) ListOperations(context.Context, int64, int, int) ([]Operation, int64, error) {
	return repository.operations, int64(len(repository.operations)), repository.operationsErr
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
	service := NewService(repository, serviceTransactor{}, projections, nil, nil)

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
	service := NewService(&serviceRepository{}, serviceTransactor{}, projections, nil, nil)

	if _, err := service.SetStatus(context.Background(), 21, "disabled"); !errors.Is(err, want) {
		t.Fatalf("error=%v", err)
	}
}

func TestResetPasswordDoesNotSynchronizeProjection(t *testing.T) {
	repository := &serviceRepository{}
	projections := &serviceProjectionWriter{}
	service := NewService(repository, serviceTransactor{}, projections, nil, nil)

	if err := service.ResetPassword(context.Background(), 21, "new-pass", "new-pass"); err != nil {
		t.Fatal(err)
	}
	if repository.passwordCalls != 1 || len(projections.items) != 0 {
		t.Fatalf("passwordCalls=%d projections=%+v", repository.passwordCalls, projections.items)
	}
}

type serviceWallets struct {
	items []commercecontract.Wallet
	err   error
	ids   []int64
}

func (*serviceWallets) Wallet(context.Context, int64) (commercecontract.Wallet, error) {
	return commercecontract.Wallet{}, nil
}
func (wallets *serviceWallets) Wallets(_ context.Context, ids []int64) ([]commercecontract.Wallet, error) {
	wallets.ids = append([]int64(nil), ids...)
	return wallets.items, wallets.err
}
func (*serviceWallets) WalletLedgers(context.Context, int64, string, int, int) ([]commercecontract.WalletLedger, int64, error) {
	return nil, 0, nil
}

func TestListAttachesWalletBalances(t *testing.T) {
	repository := &serviceRepository{users: []User{{ID: "21", Username: "reader-21"}, {ID: "22", Username: "reader-22"}}}
	wallets := &serviceWallets{items: []commercecontract.Wallet{{ReaderID: 21, RechargeCoinBalance: 88, BonusCoinBalance: 15}}}
	service := NewService(repository, serviceTransactor{}, &serviceProjectionWriter{}, nil, wallets)

	users, total, err := service.List(context.Background(), "", "", 1, 20)
	if err != nil || total != 2 || len(users) != 2 {
		t.Fatalf("users=%+v total=%d err=%v", users, total, err)
	}
	if users[0].RechargeBalance != "88" || users[0].BonusBalance != "15" {
		t.Fatalf("user0=%+v", users[0])
	}
	if users[1].RechargeBalance != "0" || users[1].BonusBalance != "0" {
		t.Fatalf("user1=%+v", users[1])
	}
}

func TestListReturnsWalletLookupFailure(t *testing.T) {
	want := errors.New("wallet unavailable")
	service := NewService(&serviceRepository{users: []User{{ID: "21"}}}, serviceTransactor{}, &serviceProjectionWriter{}, nil, &serviceWallets{err: want})
	if _, _, err := service.List(context.Background(), "", "", 1, 20); !errors.Is(err, want) {
		t.Fatalf("error=%v", err)
	}
}

func TestListOperationsRedactsPasswordAndMapsActions(t *testing.T) {
	repository := &serviceRepository{operations: []Operation{
		{ID: "1", Method: "PUT", Path: "/reader/users/21/password", Status: 200, Body: `{"password":"secret-pass","confirmPassword":"secret-pass"}`, OperatorUsername: "admin", OperatorNickname: "超管"},
		{ID: "2", Method: "PUT", Path: "/reader/users/21/status", Status: 200, Body: `{"status":"disabled"}`},
		{ID: "3", Method: "POST", Path: "/reader/users/21/membership", Status: 200, Body: `{"productId":"99","remark":"补偿会员"}`},
		{ID: "4", Method: "POST", Path: "/reader/wallets/21/adjust", Status: 200, Body: `{"coinType":"recharge","direction":"income","amount":"12","reason":"补偿钻石"}`},
	}}
	service := NewService(repository, serviceTransactor{}, &serviceProjectionWriter{}, nil, nil)
	rows, total, err := service.ListOperations(context.Background(), 21, 1, 20)
	if err != nil || total != 4 || len(rows) != 4 {
		t.Fatalf("rows=%+v total=%d err=%v", rows, total, err)
	}
	if rows[0].Action != "重置密码" || rows[0].Summary != "" || rows[0].Body != "" || rows[0].OperatorName != "admin（超管）" {
		t.Fatalf("password row=%+v", rows[0])
	}
	if rows[1].Action != "启停账号" || rows[1].Summary != "停用" {
		t.Fatalf("status row=%+v", rows[1])
	}
	if rows[2].Action != "发放会员" || rows[2].Summary != "补偿会员" {
		t.Fatalf("membership row=%+v", rows[2])
	}
	if rows[3].Action != "发放钻石" || rows[3].Summary != "钻石12 / 补偿钻石" || rows[3].Body != "" {
		t.Fatalf("wallet row=%+v", rows[3])
	}
	encoded := fmt.Sprintf("%+v", rows)
	if strings.Contains(encoded, "secret-pass") {
		t.Fatalf("password leaked: %s", encoded)
	}
}

func TestListOperationsReturnsMissingReader(t *testing.T) {
	want := errors.New("reader missing")
	service := NewService(&serviceRepository{getErr: want}, serviceTransactor{}, &serviceProjectionWriter{}, nil, nil)
	if _, _, err := service.ListOperations(context.Background(), 21, 1, 20); !errors.Is(err, want) {
		t.Fatalf("error=%v", err)
	}
}
