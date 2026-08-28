package adminpayment

import (
	"context"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/epusdt"
)

type Channel struct {
	ID                    int64
	DisplayName           string
	Provider              string
	Enabled               bool
	Currency              string
	Token                 string
	Network               string
	PIDConfigured         bool
	SecretConfigured      bool
	EPUSDTBaseURL         string
	ReaderBaseURL         string
	ConnectTimeoutMS      int
	RequestTimeoutMS      int
	UnknownReleaseMinutes int
	ArchivedAt            *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func (c Channel) Configured() bool {
	_, err := deriveEndpoints(c.EPUSDTBaseURL, c.ReaderBaseURL)
	return c.PIDConfigured && c.SecretConfigured && err == nil
}

func (c Channel) Endpoints() Endpoints {
	endpoints, _ := deriveEndpoints(c.EPUSDTBaseURL, c.ReaderBaseURL)
	return endpoints
}

type ChannelInput struct {
	DisplayName           string
	Provider              string
	Enabled               bool
	Currency              string
	Token                 string
	Network               string
	MerchantPID           *string
	Secret                *string
	EPUSDTBaseURL         string
	ReaderBaseURL         string
	ConnectTimeoutMS      int
	RequestTimeoutMS      int
	UnknownReleaseMinutes int
}

type Connectivity struct {
	ChannelID int64
	Provider  string
	Status    string
	Message   string
	CheckedAt string
}

type RuntimeConfig struct {
	ChannelID int64
	EPUSDT    epusdt.Config
	HealthURL string
	SyncURL   string
}

type Repository interface {
	List(context.Context, bool) ([]Channel, error)
	Get(context.Context, int64) (Channel, error)
	Create(context.Context, ChannelInput) (Channel, error)
	Update(context.Context, int64, ChannelInput) (Channel, error)
	Archive(context.Context, int64) error
	Runtime(context.Context) (RuntimeConfig, error)
	RuntimeForChannel(context.Context, int64) (RuntimeConfig, error)
}
