package admininvite

import "context"

type InviteCode struct {
	ID, InviterReaderID, Code, Status string
	MaxUseCount, UsedCount            string
	ExpiresAt, CreatedAt              string
}
type CreateInput struct {
	Code        string
	MaxUseCount string
	ExpiresAt   string
}
type Repository interface {
	List(context.Context, string, int, int) ([]InviteCode, int64, error)
	Create(context.Context, CreateInput) (InviteCode, error)
	SetStatus(context.Context, int64, string) (InviteCode, error)
	Delete(context.Context, int64) error
}
