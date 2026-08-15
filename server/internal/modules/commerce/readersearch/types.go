package readersearch

import (
	"errors"
	"time"
)

const MaxPageSize = 1000

var (
	ErrInvalidProjection = errors.New("invalid reader search projection")
	ErrInvalidPage       = errors.New("invalid reader search projection page")
	ErrRepositoryMissing = errors.New("reader search projection repository is required")
)

type Projection struct {
	ReaderID  int64
	Username  string
	Nickname  string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Page struct {
	Items         []Projection
	AfterReaderID int64
	Limit         int
	NextReaderID  int64
	Done          bool
}
