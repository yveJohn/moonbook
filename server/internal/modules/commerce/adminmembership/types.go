package adminmembership

import "context"

type Grant struct {
	ID, ReaderID, GrantType, StartsAt, ExpiresAt, Permanent, Status, SourceType, SourceRef, Remark, CreatedAt string
}

type MembershipProduct struct {
	ID, Name     string
	DurationDays *int
}

type Input struct {
	RequestID    string `json:"requestId"`
	ProductID    string `json:"productId"`
	Permanent    bool   `json:"permanent"`
	DurationDays string `json:"durationDays"`
	Remark       string `json:"remark"`
}

type Repository interface {
	Grant(context.Context, int64, Input) (Grant, error)
	MembershipProduct(context.Context, int64) (MembershipProduct, error)
}

type Transactor interface {
	Within(context.Context, func(context.Context) error) error
}
