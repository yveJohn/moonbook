package contract

import "context"

type Account struct {
	ID       int64
	Username string
	Nickname string
	Status   string
}

type AccountReader interface {
	Account(context.Context, int64) (Account, error)
}

type AccountLocker interface {
	LockAccount(context.Context, int64) (Account, error)
}

type AccountPage struct {
	Items  []Account
	NextID int64
	Done   bool
}

type AccountSnapshotPager interface {
	AccountSnapshots(context.Context, int64, int) (AccountPage, error)
}
