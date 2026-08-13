package books

import "time"

type Category struct {
	ID   int64
	Code string
	Name string
}

type Book struct {
	ID                   int64
	WorkDirection        *string
	PrimaryCategory      Category
	LegacyCoverURL       string
	BookName             string
	AuthorID             int64
	AuthorName           string
	Description          string
	Score                string
	BookStatus           string
	PublishStatus        string
	SourceType           string
	Featured             bool
	FeaturedSort         int
	FeaturedNote         string
	VisitCount           int64
	LikeCount            int
	WordCount            int
	CommentCount         int
	YesterdayBuy         int
	LastChapterID        *int64
	LastChapterName      *string
	LastChapterUpdatedAt *time.Time
	ChargeMode           string
	FixedPriceCoin       *int64
	SubCategories        []Category
	Tags                 []string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type Input struct {
	WorkDirection    *string
	CategoryCode     string
	LegacyCoverURL   string
	BookName         string
	AuthorID         int64
	Description      string
	Score            string
	BookStatus       string
	PublishStatus    string
	SourceType       string
	Featured         bool
	FeaturedSort     int
	FeaturedNote     string
	ChargeMode       string
	FixedPriceCoin   *int64
	SubCategoryCodes []string
	Tags             []string
}

type Filter struct {
	Page          int
	PageSize      int
	Keyword       string
	CategoryCode  string
	BookStatus    string
	PublishStatus string
	SourceType    string
	ChargeMode    string
}

type Page struct {
	Items    []Book
	Total    int64
	Page     int
	PageSize int
}
