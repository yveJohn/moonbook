package adminuser

import "context"

type User struct {
	ID, Username, Nickname, Status, PasswordAlgorithm string
	LastLoginAt, CreatedAt, UpdatedAt                 string
}
type Repository interface {
	List(context.Context, string, string, int, int) ([]User, int64, error)
	Get(context.Context, int64) (User, error)
	SetStatus(context.Context, int64, string) (User, error)
	ResetPassword(context.Context, int64, string, string) error
}

type Transactor interface {
	Within(context.Context, func(context.Context) error) error
}
