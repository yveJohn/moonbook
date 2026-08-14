package candidate

import "context"

type Candidate struct {
	ID, SourceID, SourceName, BoardID, BoardName                                 string
	ForumThreadID, ThreadTitle, DisplayTitle, ThreadURL, AuthorID                string
	Status, ImportTaskID, TargetBookID, LastImportFloorID                        string
	LastImportPageNo, FollowCount                                                string
	LastFollowTime, FollowFailReason, DiscoverTime, Remark, CreatedAt, UpdatedAt string
}

type BoardTarget struct {
	SourceID, SourceName, BoardID, BoardName string
	BoardURL, BoardURLTemplate, UserAgent    string
	Enabled                                  bool
}

type Discovered struct {
	ForumThreadID, ThreadTitle, ThreadURL, AuthorID string
}

type DiscoverResult struct {
	DiscoveredCount, InsertedCount, UpdatedCount int
}

type Repository interface {
	List(context.Context, string, string, string, string, int, int) ([]Candidate, int64, error)
	Get(context.Context, int64) (Candidate, error)
	Transition(context.Context, []int64, string, string) error
	Delete(context.Context, []int64) error
	GetBoardTarget(context.Context, string) (BoardTarget, error)
	UpsertDiscovered(context.Context, BoardTarget, []Discovered) (DiscoverResult, error)
}
