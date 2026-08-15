package public

import (
	"time"

	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
)

type Category = novelcontract.Category
type Book = novelcontract.Book

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
	Access     commercecontract.AccessResult
	PrevID     *int64
	NextID     *int64
}

type ChapterContentMeta struct {
	Version  int
	SHA256   string
	ByteSize int64
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
	Status      commercecontract.AccessResult
	Liked       bool
	InBookshelf bool
	History     *History
}
