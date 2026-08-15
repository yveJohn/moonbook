package admininvite

import "context"

type InviteCode struct {
	ID, InviterReaderID, Code, Status string
	MaxUseCount, UsedCount            string
	ExpiresAt, Remark, CreatedAt      string
}
type CreateInput struct {
	Code, Status, MaxUseCount, ExpiresAt, Remark string
}
type UpdateInput struct {
	Code, Status, MaxUseCount, ExpiresAt, Remark string
}
type Repository interface {
	List(context.Context, string, int, int) ([]InviteCode, int64, error)
	Create(context.Context, CreateInput) (InviteCode, error)
	Update(context.Context, int64, UpdateInput) (InviteCode, error)
	SetStatus(context.Context, int64, string) (InviteCode, error)
	Delete(context.Context, int64) error
}
