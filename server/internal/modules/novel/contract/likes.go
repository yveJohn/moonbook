package contract

import "context"

type LikeBookLocker interface {
	LockPublishedBook(context.Context, int64) error
}

type LikeSummaryWriter interface {
	SetLikeCount(context.Context, int64, int64) error
}
