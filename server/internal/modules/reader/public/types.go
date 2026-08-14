package public

import "time"
import "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/catalog"

type Category struct{ Code, Name string }
type Book struct {
	ID                                                                            int64
	Name, Author, Description, CategoryCode, CategoryName, BookStatus, ChargeMode string
	SubCategories                                                                 []Category
	WordCount, LikeCount                                                          int
	VisitCount                                                                    int64
	FixedPriceCoin                                                                *int64
	LastChapterID                                                                 *int64
	LastChapterName                                                               *string
	LastChapterUpdatedAt                                                          *time.Time
	Featured                                                                      bool
	FeaturedNote                                                                  string
}
type Chapter struct {
	ID, BookID int64
	No         int
	Name       string
	WordCount  int
	UpdatedAt  time.Time
	IsVIP      bool
	Price      int64
	ChargeMode string
	BookName   string
	Access     catalog.AccessResult
	PrevID     *int64
	NextID     *int64
}

type History struct {
	ID, BookID, ChapterID     int64
	ChapterNo                 int
	ChapterName, PositionType string
	PositionValue             int
	ProgressPercent           string
	LastReadAt                time.Time
}

type BookDetail struct {
	Book
	Status      catalog.AccessResult
	Liked       bool
	InBookshelf bool
	History     *History
}
