package contract

import "context"

type TargetSnapshot struct {
	Type     string
	TargetID int64
	BookID   int64
	Name     string
	Enabled  bool
}

type ProductTargetReader interface {
	ProductTarget(context.Context, string, int64) (TargetSnapshot, error)
}

type PurchaseSnapshot struct {
	Type      string
	TargetID  int64
	BookID    int64
	Name      string
	WordCount int
	Enabled   bool
}

type PurchaseSnapshotReader interface {
	LockPurchaseSnapshot(context.Context, string, int64) (PurchaseSnapshot, error)
}
