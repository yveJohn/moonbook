package recharge

import (
	"context"
	"errors"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/epusdt"
)

type Gateway interface {
	Create(context.Context, epusdt.CreateRequest) (epusdt.CreateResponse, error)
}

type GatewaySnapshot struct {
	ChannelID     int64
	CredentialRef string
	MerchantPID   string
}

type Service struct {
	Repo        Repository
	Gateway     Gateway
	Snapshot    GatewaySnapshot
	ConfigError error
	LoadGateway GatewayLoader
}

type GatewayLoader func(context.Context) (Gateway, GatewaySnapshot, error)

func NewService(repo Repository, gateway Gateway, snapshot GatewaySnapshot, configError error) *Service {
	return &Service{Repo: repo, Gateway: gateway, Snapshot: snapshot, ConfigError: configError}
}

func NewDynamicService(repo Repository, loader GatewayLoader) *Service {
	return &Service{Repo: repo, LoadGateway: loader}
}
func (s *Service) Catalog(ctx context.Context) (Catalog, error) { return s.Repo.Catalog(ctx) }
func (s *Service) Quote(ctx context.Context, amount int64) (Quote, error) {
	return s.Repo.Quote(ctx, amount)
}
func (s *Service) CreateOrder(ctx context.Context, req CreateRequest) (Order, error) {
	prepared, err := s.Repo.Prepare(ctx, req)
	if err != nil {
		return Order{}, err
	}
	gateway, snapshot, configErr := s.Gateway, s.Snapshot, s.ConfigError
	if s.LoadGateway != nil {
		gateway, snapshot, configErr = s.LoadGateway(ctx)
	}
	if configErr != nil {
		return Order{}, configErr
	}
	if gateway == nil || snapshot.CredentialRef == "" || snapshot.MerchantPID == "" {
		return Order{}, errors.New("EPUSDT payment configuration is unavailable")
	}
	prepared.ChannelID = snapshot.ChannelID
	prepared.CredentialRef = snapshot.CredentialRef
	prepared.MerchantPIDSnapshot = snapshot.MerchantPID
	started, err := s.Repo.Start(ctx, prepared)
	if err != nil || !started.Created {
		return started.Order, err
	}

	response, gatewayErr := gateway.Create(ctx, epusdt.CreateRequest{OrderID: started.Order.OrderNo, Amount: started.Order.PriceUSDT})
	if gatewayErr == nil {
		completed, completeErr := s.Repo.Complete(ctx, started.Order.ID, response)
		if completeErr != nil {
			return started.Order, persistenceError(started.Order, completeErr)
		}
		return completed, nil
	}
	failure := gatewayFailure(gatewayErr)
	failed, failErr := s.Repo.Fail(ctx, started.Order.ID, failure)
	if failErr != nil {
		return started.Order, persistenceError(started.Order, failErr)
	}
	return failed, nil
}

func persistenceError(order Order, cause error) error {
	return &GatewayPersistenceError{OrderID: order.ID, OrderNo: order.OrderNo, cause: cause}
}
func (s *Service) GetOrder(ctx context.Context, readerID int64, orderID string) (Order, error) {
	return s.Repo.GetOrder(ctx, readerID, orderID)
}

func gatewayFailure(err error) *epusdt.GatewayError {
	var failure *epusdt.GatewayError
	if errors.As(err, &failure) {
		return failure
	}
	return &epusdt.GatewayError{
		Code:    "GATEWAY_CALL_FAILED",
		Class:   epusdt.FailureUncertain,
		Summary: "EPUSDT create result could not be determined",
	}
}
