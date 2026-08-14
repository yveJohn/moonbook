package candidate

import "context"

type Candidate struct {
	ID, SourceID, SourceName, BoardID, BoardName                                 string
	ForumThreadID, ThreadTitle, DisplayTitle, ThreadURL, AuthorID                string
	Status, ImportTaskID, TargetBookID, LastImportFloorID                        string
	LastImportPageNo, FollowCount                                                string
	LastFollowTime, FollowFailReason, DiscoverTime, Remark, CreatedAt, UpdatedAt string
}

type Repository interface {
	List(context.Context, string, string, string, string, int, int) ([]Candidate, int64, error)
	Get(context.Context, int64) (Candidate, error)
	Transition(context.Context, []int64, string, string) error
	Delete(context.Context, []int64) error
}
