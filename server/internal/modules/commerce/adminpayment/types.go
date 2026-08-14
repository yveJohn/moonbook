package adminpayment

import "context"

type Channel struct {
	ID         int64
	Provider   string
	Enabled    bool
	Currency   string
	Token      string
	Network    string
	Configured bool
}

type Connectivity struct {
	ChannelID int64
	Provider  string
	Status    string
	Message   string
	CheckedAt string
}

type Repository interface {
	List(ctx context.Context) ([]Channel, error)
	SetEnabled(ctx context.Context, id int64, enabled bool) (Channel, error)
	Check(ctx context.Context, id int64) (Connectivity, error)
}
