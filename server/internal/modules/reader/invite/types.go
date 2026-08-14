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

// Repository owns the registration transaction. Implementations must lock the
// invite row before validating or incrementing its usage count.
type Repository interface {
	RegisterWithInvite(context.Context, Registration) (auth.ReaderAccount, error)
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
