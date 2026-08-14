package objectstore

import (
	"errors"
	"net/http"
	"time"
)

var ErrActiveObjectNotFound = errors.New("active object not found")

const (
	KindChapterContent = "chapter_content"
	KindChapterClean   = "chapter_clean"
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

type UploadOptions struct {
	Source            string
	SourceFingerprint string
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

func DetectCover(data []byte) (contentType, extension string, ok bool) {
	switch http.DetectContentType(data) {
	case "image/jpeg":
		return "image/jpeg", "jpg", true
	case "image/png":
		return "image/png", "png", true
	case "image/gif":
		return "image/gif", "gif", true
	case "image/webp":
		return "image/webp", "webp", true
	default:
		return "", "", false
	}
}
