package importtask

import (
	"context"
	"time"
)

type Task struct {
	ID, CandidateID, SourceID, SourceName, BoardID, BoardName                                                    string
	ForumThreadID, ThreadTitle, DisplayTitle, ThreadURL, TargetBookID                                            string
	ImportMode, MergeStrategy, Status, QualityStatus                                                             string
	TotalChapterCount, ImportedChapterCount, EmptyChapterCount, DuplicateChapterCount, AttemptCount, MaxAttempts string
	QualitySummary, FailReason, OperatorName, StartTime, EndTime, CreatedAt, UpdatedAt                           string
	requestInterval                                                                                              time.Duration
	sourceUserAgent, sourceCookie                                                                                string
}
type CreateInput struct {
	CandidateID    string `json:"candidateId"`
	DisplayTitle   string `json:"displayTitle"`
	TargetBookID   string `json:"targetBookId"`
	ImportMode     string `json:"importMode"`
	MergeStrategy  string `json:"mergeStrategy"`
	OperatorName   string `json:"operatorName"`
	IdempotencyKey string `json:"idempotencyKey"`
}
type Repository interface {
	List(context.Context, string, string, string, string, int, int) ([]Task, int64, error)
	Get(context.Context, int64) (Task, error)
	Create(context.Context, CreateInput) (Task, error)
	Retry(context.Context, int64, CreateInput) (Task, error)
	Cancel(context.Context, int64) error
}
