package fetchlog

import "context"

type Log struct {
	ID, TaskID, SourceID, SourceName, BoardID, BoardName, ThreadURL, RequestURL  string
	Stage, Status, HTTPStatus, ResponseBytes, ElapsedMs, ItemCount, TargetBookID string
	Message, DetailJSON, CreatedAt                                               string
}
type Repository interface {
	List(context.Context, string, string, string, string, int, int) ([]Log, int64, error)
	Get(context.Context, int64) (Log, error)
}
