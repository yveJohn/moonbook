package recharge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/epusdt"
)

type serviceRepositoryStub struct {
	prepared      PreparedOrder
	start         StartResult
	completed     Order
	failed        Order
	prepareErr    error
	startErr      error
	completeErr   error
	failErr       error
	prepareCalls  int
	startCalls    int
	completeCalls int
	failCalls     int
	failure       *epusdt.GatewayError
}

func (stub *serviceRepositoryStub) Catalog(context.Context) (Catalog, error) { return Catalog{}, nil }
func (stub *serviceRepositoryStub) Quote(context.Context, int64) (Quote, error) {
	return Quote{}, nil
}
func (stub *serviceRepositoryStub) Prepare(_ context.Context, request CreateRequest) (PreparedOrder, error) {
	stub.prepareCalls++
	prepared := stub.prepared
	prepared.ReaderID = request.ReaderID
	prepared.RequestID = request.RequestID
	prepared.ProductID = request.ProductID
	return prepared, stub.prepareErr
}
func (stub *serviceRepositoryStub) Start(_ context.Context, prepared PreparedOrder) (StartResult, error) {
	stub.startCalls++
	stub.prepared = prepared
	return stub.start, stub.startErr
}
func (stub *serviceRepositoryStub) Complete(_ context.Context, _ string, _ epusdt.CreateResponse) (Order, error) {
	stub.completeCalls++
	return stub.completed, stub.completeErr
}
func (stub *serviceRepositoryStub) Fail(_ context.Context, _ string, failure *epusdt.GatewayError) (Order, error) {
	stub.failCalls++
	stub.failure = failure
	return stub.failed, stub.failErr
}
func (stub *serviceRepositoryStub) GetOrder(context.Context, int64, string) (Order, error) {
	return Order{}, nil
}

type gatewayStub struct {
	response epusdt.CreateResponse
	err      error
	calls    int
	request  epusdt.CreateRequest
}

func (stub *gatewayStub) Create(_ context.Context, request epusdt.CreateRequest) (epusdt.CreateResponse, error) {
	stub.calls++
	stub.request = request
	return stub.response, stub.err
}

func preparedFixture() PreparedOrder {
	return PreparedOrder{
		SourceType:    "custom",
		DiamondAmount: 100,
		PriceUSDT:     "1.00",
		Provider:      "epusdt",
		Currency:      "usd",
		Token:         "usdt",
		Network:       "tron",
	}
}

func TestCreateOrderDoesNotCallGatewayForExistingOrBlockingOrder(t *testing.T) {
	for _, status := range []string{"pending", "creating", "gateway_unknown"} {
		t.Run(status, func(t *testing.T) {
			repository := &serviceRepositoryStub{prepared: preparedFixture(), start: StartResult{Order: Order{ID: "11", OrderNo: "RC11", Status: status}}}
			gateway := &gatewayStub{}
			service := NewService(repository, gateway, GatewaySnapshot{CredentialRef: "primary", MerchantPID: "merchant"}, nil)

			order, err := service.CreateOrder(context.Background(), CreateRequest{ReaderID: 7, CustomDiamondAmount: "100", RequestID: "same"})
			if err != nil || order.Status != status || gateway.calls != 0 || repository.completeCalls != 0 || repository.failCalls != 0 {
				t.Fatalf("order=%+v err=%v gateway=%d complete=%d fail=%d", order, err, gateway.calls, repository.completeCalls, repository.failCalls)
			}
		})
	}
}

func TestCreateOrderCallsGatewayOnceAndCompletes(t *testing.T) {
	expires := time.Now().Add(20 * time.Minute).UTC()
	repository := &serviceRepositoryStub{
		prepared:  preparedFixture(),
		start:     StartResult{Created: true, Order: Order{ID: "12", OrderNo: "RC12", PriceUSDT: "1.00", Status: "creating"}},
		completed: Order{ID: "12", OrderNo: "RC12", Status: "pending"},
	}
	gateway := &gatewayStub{response: epusdt.CreateResponse{TradeID: "trade", OrderID: "RC12", Amount: "1.00", Status: 1, ExpirationTime: expires}}
	service := NewService(repository, gateway, GatewaySnapshot{CredentialRef: "primary", MerchantPID: "merchant"}, nil)

	order, err := service.CreateOrder(context.Background(), CreateRequest{ReaderID: 7, CustomDiamondAmount: "100", RequestID: "new"})
	if err != nil || order.Status != "pending" || gateway.calls != 1 || repository.completeCalls != 1 || repository.failCalls != 0 {
		t.Fatalf("order=%+v err=%v gateway=%d complete=%d fail=%d", order, err, gateway.calls, repository.completeCalls, repository.failCalls)
	}
	if gateway.request.OrderID != "RC12" || gateway.request.Amount != "1.00" || repository.prepared.CredentialRef != "primary" || repository.prepared.MerchantPIDSnapshot != "merchant" {
		t.Fatalf("gateway request=%+v prepared=%+v", gateway.request, repository.prepared)
	}
}

func TestCreateOrderPersistsGatewayFailureClass(t *testing.T) {
	for _, test := range []struct {
		name       string
		failure    *epusdt.GatewayError
		wantStatus string
	}{
		{name: "definite", failure: &epusdt.GatewayError{Code: "GATEWAY_REJECTED", Class: epusdt.FailureDefinite, Summary: "rejected"}, wantStatus: "create_failed"},
		{name: "uncertain", failure: &epusdt.GatewayError{Code: "REQUEST_TIMEOUT", Class: epusdt.FailureUncertain, Summary: "timed out"}, wantStatus: "gateway_unknown"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &serviceRepositoryStub{
				prepared: preparedFixture(),
				start:    StartResult{Created: true, Order: Order{ID: "13", OrderNo: "RC13", PriceUSDT: "1.00", Status: "creating"}},
				failed:   Order{ID: "13", Status: test.wantStatus},
			}
			gateway := &gatewayStub{err: test.failure}
			service := NewService(repository, gateway, GatewaySnapshot{CredentialRef: "primary", MerchantPID: "merchant"}, nil)

			order, err := service.CreateOrder(context.Background(), CreateRequest{ReaderID: 7, CustomDiamondAmount: "100", RequestID: test.name})
			if err != nil || order.Status != test.wantStatus || gateway.calls != 1 || repository.failCalls != 1 || repository.failure != test.failure {
				t.Fatalf("order=%+v err=%v gateway=%d fail=%d failure=%+v", order, err, gateway.calls, repository.failCalls, repository.failure)
			}
		})
	}
}

func TestCreateOrderNeverRetriesGatewayWhenPersistenceFails(t *testing.T) {
	for _, test := range []struct {
		name       string
		gatewayErr error
		complete   bool
	}{
		{name: "complete", complete: true},
		{name: "fail", gatewayErr: &epusdt.GatewayError{Code: "REQUEST_TIMEOUT", Class: epusdt.FailureUncertain, Summary: "timed out"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			persistErr := errors.New("database unavailable")
			repository := &serviceRepositoryStub{prepared: preparedFixture(), start: StartResult{Created: true, Order: Order{ID: "14", OrderNo: "RC14", PriceUSDT: "1.00", Status: "creating"}}}
			if test.complete {
				repository.completeErr = persistErr
			} else {
				repository.failErr = persistErr
			}
			gateway := &gatewayStub{err: test.gatewayErr}
			service := NewService(repository, gateway, GatewaySnapshot{CredentialRef: "primary", MerchantPID: "merchant"}, nil)

			order, err := service.CreateOrder(context.Background(), CreateRequest{ReaderID: 7, CustomDiamondAmount: "100", RequestID: test.name})
			var persistenceErr *GatewayPersistenceError
			if !errors.Is(err, persistErr) || !errors.As(err, &persistenceErr) || persistenceErr.OrderID != "14" || persistenceErr.OrderNo != "RC14" || err.Error() != "persist recharge gateway result for order RC14 failed" || order.ID != "14" || gateway.calls != 1 {
				t.Fatalf("order=%+v err=%v gateway=%d", order, err, gateway.calls)
			}
		})
	}
}

func TestCreateOrderRejectsConfigurationBeforeStart(t *testing.T) {
	configErr := errors.New("EPUSDT configuration unavailable")
	repository := &serviceRepositoryStub{prepared: preparedFixture()}
	gateway := &gatewayStub{}
	service := NewService(repository, gateway, GatewaySnapshot{}, configErr)

	_, err := service.CreateOrder(context.Background(), CreateRequest{ReaderID: 7, CustomDiamondAmount: "100", RequestID: "config"})
	if !errors.Is(err, configErr) || repository.prepareCalls != 1 || repository.startCalls != 0 || gateway.calls != 0 {
		t.Fatalf("err=%v prepare=%d start=%d gateway=%d", err, repository.prepareCalls, repository.startCalls, gateway.calls)
	}
}
