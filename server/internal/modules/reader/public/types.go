package public

import "time"
import "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/catalog"

type Category struct{ Code, Name string }
type Book struct {
	ID                                                                            int64
	Name, Author, Description, CategoryCode, CategoryName, BookStatus, ChargeMode string
	SubCategories                                                                 []Category
	WordCount, LikeCount                                                          int
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
}
