package adminuser

import "context"

type User struct {
	ID, Username, Nickname, Status, PasswordAlgorithm string
	LastLoginAt, CreatedAt, UpdatedAt                 string
	RechargeBalance, BonusBalance                     string
}

type Operation struct {
	ID, CreatedAt, Method, Path                    string
	Status                                         int64
	Body, ErrorMessage                             string
	OperatorID, OperatorUsername, OperatorNickname string
	OperatorName, Action, Summary                  string
	Source, EventType, TargetID                    string
}

type Repository interface {
	List(context.Context, string, string, int, int) ([]User, int64, error)
	Get(context.Context, int64) (User, error)
	SetStatus(context.Context, int64, string) (User, error)
	ResetPassword(context.Context, int64, string, string) error
	ListOperations(context.Context, int64, int, int) ([]Operation, int64, error)
}

type Transactor interface {
	Within(context.Context, func(context.Context) error) error
}
