package invite

import (
	"context"
	"database/sql"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
)

var (
	ErrInviteRequired = auth.ErrInvalidCredentials
)

// Repository owns only Reader registration facts. Every method must join the
// transaction bound to the context and must not commit independently.
type Repository interface {
	LockInvite(context.Context, string) (InviteCode, error)
	CreateAccount(context.Context, Registration, int64) (auth.ReaderAccount, error)
	IncrementInviteUsage(context.Context, int64) error
	CreateAutomaticInviteCode(context.Context, int64) error
	CreateInviteRelation(context.Context, int64, int64, int64) (int64, error)
}

type Transactor interface {
	Within(context.Context, func(context.Context) error) error
}

type Registration struct {
	Username   string
	Password   string
	Nickname   string
	InviteCode string
}

type SQLRepository struct{ DB *sql.DB }

// InviteCode is exposed for repository tests and migration tooling.
type InviteCode struct {
	ID, InviterReaderID int64
	Code, Status        string
	MaxUseCount         *int
	UsedCount           int
	ExpiresAt           *time.Time
}
