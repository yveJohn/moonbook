package jobmonitor

import (
	"context"
	"errors"
)

var (
	ErrInvalidArgument = errors.New("invalid argument")
	ErrNotFound        = errors.New("platform job not found")
)

type Job struct {
	ID, Module, JobType, Status, LeaseOwner, LastErrorCode, LastErrorMessage string
	AttemptCount, MaxAttempts                                                int
	AvailableAt, LeaseExpiresAt, CreatedAt, UpdatedAt, FinishedAt            string
}

type Attempt struct {
	ID, JobID, WorkerID, Outcome, ErrorCode, ErrorMessage string
	AttemptNumber                                         int
	StartedAt, FinishedAt                                 string
}

type Detail struct {
	Job
	Attempts []Attempt
}

type Filters struct {
	Module, JobType, Status, LeaseOwner, From, To string
}

type Repository interface {
	List(context.Context, Filters, int, int) ([]Job, int64, error)
	Get(context.Context, string) (Detail, error)
}
