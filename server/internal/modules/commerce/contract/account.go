package contract

import (
	"context"
	"time"
)

type ReaderSearchProjection struct {
	ReaderID int64
	Username string
	Nickname string
	Status   string
}

type ReaderSearchProjectionPage struct {
	Items        []ReaderSearchProjection
	NextReaderID int64
	Done         bool
}

type ReaderSearchProjectionWriter interface {
	UpsertReaderSearchProjection(context.Context, ReaderSearchProjection) error
	DeleteReaderSearchProjection(context.Context, int64) error
}

type ReaderSearchProjectionReader interface {
	ReaderSearchProjections(context.Context, int64, int) (ReaderSearchProjectionPage, error)
}

type EntitlementSummary struct {
	ReaderID            int64
	BookIDs             []int64
	MembershipActive    bool
	MembershipPermanent bool
	MembershipExpiresAt *time.Time
}

type MembershipProduct struct {
	ID             int64
	Name           string
	PriceCoin      int64
	AllowBonusCoin bool
	DurationDays   *int
	SaleStatus     string
	SortOrder      int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ReaderAccountSummary interface {
	Entitlements(context.Context, int64) (EntitlementSummary, error)
	MembershipProducts(context.Context) ([]MembershipProduct, error)
}
