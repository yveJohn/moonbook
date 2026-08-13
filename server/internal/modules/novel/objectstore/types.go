package objectstore

import "time"

const (
	KindChapterContent = "chapter_content"
	KindBookCover      = "book_cover"

	StateUploading = "uploading"
	StateVerified  = "verified"
	StateActive    = "active"
	StateOrphaned  = "orphaned"
	StateFailed    = "failed"
	StateDeleting  = "deleting"
	StateDeleted   = "deleted"
)

type Target struct {
	Kind      string
	BookID    int64
	OwnerID   int64
	Extension string
}

type Object struct {
	ID          int64
	Target      Target
	Version     int
	Key         string
	SHA256      string
	ByteSize    int64
	ContentType string
	State       string
	CreatedAt   time.Time
}

type BlobStat struct {
	Key         string
	SHA256      string
	ByteSize    int64
	ContentType string
}

type CollectResult struct {
	Examined int
	Deleted  int
	Failed   int
}
