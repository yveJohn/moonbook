package adminfeedback

import "context"

type Feedback struct {
	ID, ReaderID, BookID, ChapterID                              string
	ReaderUsername, Content, Status, Reply, CreatedAt, RepliedAt string
}
type Repository interface {
	List(context.Context, string, string, int, int) ([]Feedback, int64, error)
	Get(context.Context, int64) (Feedback, error)
	Reply(context.Context, int64, string) (Feedback, error)
}
