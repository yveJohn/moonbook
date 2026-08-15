package contract

import (
	"context"
	"time"
)

type Category struct {
	Code string
	Name string
}

type Book struct {
	ID                   int64
	Name                 string
	Author               string
	Description          string
	CategoryCode         string
	CategoryName         string
	BookStatus           string
	ChargeMode           string
	SubCategories        []Category
	WordCount            int
	LikeCount            int
	VisitCount           int64
	FixedPriceCoin       *int64
	LastChapterID        *int64
	LastChapterName      *string
	LastChapterUpdatedAt *time.Time
	Featured             bool
	FeaturedNote         string
}

type Chapter struct {
	ID         int64
	BookID     int64
	Number     int
	Name       string
	WordCount  int
	UpdatedAt  time.Time
	VIP        bool
	PriceCoin  int64
	ChargeMode string
	BookName   string
	PreviousID *int64
	NextID     *int64
}

type ChapterContent struct {
	Chapter Chapter
	Text    string
	Version int
	SHA256  string
	Bytes   int64
}

type PublicBookReader interface {
	FeaturedBooks(context.Context) ([]Book, error)
	RandomBooks(context.Context) ([]Book, error)
	Books(context.Context, string, string, string, int, int) ([]Book, int64, error)
	Categories(context.Context, string) ([]Category, error)
	Book(context.Context, int64) (Book, error)
}

type PublicChapterReader interface {
	Chapters(context.Context, int64) ([]Chapter, error)
	Chapter(context.Context, int64) (Chapter, error)
	ChapterContent(context.Context, int64) (ChapterContent, error)
}

type SEOConfig struct {
	Enabled                  bool
	IndexingEnabled          bool
	SitemapEnabled           bool
	SiteName                 string
	SiteURL                  string
	DefaultDescription       string
	HomeTitle                string
	HomeDescription          string
	BooksTitleTemplate       string
	BooksDescriptionTemplate string
	BookTitleTemplate        string
	BookDescriptionTemplate  string
}

type SEOReader interface {
	SEO(context.Context) (SEOConfig, error)
	Robots(context.Context) (string, error)
	Sitemap(context.Context, int) (string, error)
}

type BookDisplay struct {
	ID           int64
	Name         string
	Author       string
	Description  string
	CategoryCode string
	CategoryName string
	WordCount    int
	LikeCount    int
	Published    bool
	Found        bool
}

type ChapterDisplay struct {
	ID      int64
	BookID  int64
	Number  int
	Name    string
	Enabled bool
	Found   bool
}

type DisplayRequest struct {
	BookIDs    []int64
	ChapterIDs []int64
}

type DisplayBatch struct {
	Books    []BookDisplay
	Chapters []ChapterDisplay
}

// BatchDisplay returns Books and Chapters in their corresponding input order.
type DisplayReader interface {
	BatchDisplay(context.Context, DisplayRequest) (DisplayBatch, error)
}
