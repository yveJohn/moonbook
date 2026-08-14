package chapters

import "time"

type Chapter struct {
	ID            int64
	BookID        int64
	ChapterNo     int
	ChapterName   string
	WordCount     int
	IsVIP         bool
	BookPriceCoin int64
	ChapterStatus string
	AICleanStatus string
	SourceType    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Input struct {
	BookID        int64
	ChapterNo     *int
	ChapterName   string
	IsVIP         bool
	BookPriceCoin int64
	ChapterStatus string
	AICleanStatus string
	SourceType    string
	Content       *string
}

type Filter struct {
	Page          int
	PageSize      int
	BookID        int64
	Keyword       string
	ChapterStatus string
	AICleanStatus string
}

type Page struct {
	Items    []Chapter
	Total    int64
	Page     int
	PageSize int
}

type Content struct {
	Chapter Chapter
	Text    string
	Version int
	SHA256  string
	Bytes   int64
}
