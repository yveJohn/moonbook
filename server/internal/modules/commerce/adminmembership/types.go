package adminmembership

import "context"

type Grant struct {
	ID, ReaderID, GrantType, StartsAt, ExpiresAt, Permanent, Status, SourceType, SourceRef, Remark, CreatedAt string
}

type Input struct {
	RequestID    string `json:"requestId"`
	Permanent    bool   `json:"permanent"`
	DurationDays string `json:"durationDays"`
	Remark       string `json:"remark"`
}

type Repository interface {
	Grant(context.Context, int64, Input) (Grant, error)
}
