package txtimport

import "context"

const MaxFileBytes int64 = (16 << 20) - 1

type Task struct {
	ID, TargetBookID, OriginalFilename, ObjectKey, ObjectSHA256, ObjectByteSize                   string
	Status, QualityStatus, QualitySummary                                                         string
	TotalChapterCount, ImportedChapterCount, EmptyChapterCount, DuplicateChapterCount             string
	FailReason, OperatorName, AttemptCount, MaxAttempts, StartTime, EndTime, CreatedAt, UpdatedAt string
}

type ObjectMeta struct {
	Key, SHA256, ContentType string
	ByteSize                 int64
}

type CreateInput struct {
	TargetBookID, OriginalFilename, OperatorName string
	Object                                       ObjectMeta
}

type Repository interface {
	List(context.Context, string, string, int, int) ([]Task, int64, error)
	Get(context.Context, int64) (Task, error)
	Create(context.Context, CreateInput) (Task, error)
	Retry(context.Context, int64) (Task, error)
	Cancel(context.Context, int64) error
}

type FileStore interface {
	Put(context.Context, string, []byte, string) (ObjectMeta, error)
	Get(context.Context, ObjectMeta) ([]byte, error)
}
