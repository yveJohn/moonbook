package metadata

import "time"

const (
	CategoryKindPrimary = "primary"
	CategoryKindSub     = "sub"

	AuthorStatusPending = "pending"
	AuthorStatusActive  = "active"
	AuthorStatusBlocked = "blocked"
)

type Category struct {
	ID            int64
	Code          string
	Name          string
	Kind          string
	WorkDirection *string
	Sort          int
	Enabled       bool
	Source        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Author struct {
	ID                 int64
	PenName            string
	NormalizedName     string
	Status             string
	WorkDirection      *string
	Source             string
	LegacyAuthorID     *int64
	LegacyBookAuthorID *int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type CategoryInput struct {
	Code          string
	Name          string
	Kind          string
	WorkDirection *string
	Sort          int
	Enabled       bool
}

type AuthorInput struct {
	PenName       string
	Status        string
	WorkDirection *string
}

type ListFilter struct {
	Page     int
	PageSize int
	Keyword  string
	Kind     string
	Status   string
	Enabled  *bool
}

type Page[T any] struct {
	Items    []T
	Total    int64
	Page     int
	PageSize int
}
