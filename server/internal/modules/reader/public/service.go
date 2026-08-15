package public

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

type Service struct {
	db       *sql.DB
	books    novelcontract.PublicBookReader
	chapters novelcontract.PublicChapterReader
	seo      novelcontract.SEOReader
	access   commercecontract.AccessReader
}

func NewService(db *sql.DB, books novelcontract.PublicBookReader, chapters novelcontract.PublicChapterReader, seo novelcontract.SEOReader, access commercecontract.AccessReader) *Service {
	return &Service{db: db, books: books, chapters: chapters, seo: seo, access: access}
}

func notFound(message string) error {
	return apperror.New(apperror.CodeNotFound, http.StatusNotFound, message)
}

func (service *Service) Featured(ctx context.Context) ([]Book, error) {
	if service == nil || service.books == nil {
		return nil, novelcontract.ErrUnavailable
	}
	return service.books.FeaturedBooks(ctx)
}

func (service *Service) Random(ctx context.Context) ([]Book, error) {
	if service == nil || service.books == nil {
		return nil, novelcontract.ErrUnavailable
	}
	return service.books.RandomBooks(ctx)
}

func (service *Service) List(ctx context.Context, keyword, category, subcategory string, page, size int) ([]Book, int64, error) {
	if service == nil || service.books == nil {
		return nil, 0, novelcontract.ErrUnavailable
	}
	return service.books.Books(ctx, keyword, category, subcategory, page, size)
}

func (service *Service) Categories(ctx context.Context, kind string) ([]Category, error) {
	if service == nil || service.books == nil {
		return nil, novelcontract.ErrUnavailable
	}
	return service.books.Categories(ctx, kind)
}

func (service *Service) Get(ctx context.Context, id int64) (Book, error) {
	if service == nil || service.books == nil {
		return Book{}, novelcontract.ErrUnavailable
	}
	book, err := service.books.Book(ctx, id)
	if errors.Is(err, novelcontract.ErrBookNotFound) {
		return Book{}, notFound("书籍不存在")
	}
	return book, err
}

func (service *Service) Detail(ctx context.Context, id int64, readerID *int64) (BookDetail, error) {
	book, err := service.Get(ctx, id)
	if err != nil {
		return BookDetail{}, err
	}
	status, err := service.BookStatus(ctx, book, readerID)
	if err != nil {
		return BookDetail{}, err
	}
	detail := BookDetail{Book: book, Status: status}
	if readerID == nil {
		return detail, nil
	}
	if service.db == nil {
		return BookDetail{}, errors.New("reader relationship repository is unavailable")
	}
	if err = service.db.QueryRowContext(ctx, `SELECT
		EXISTS(SELECT 1 FROM reader_book_likes WHERE reader_id=$1 AND book_id=$2),
		EXISTS(SELECT 1 FROM reader_bookshelf_entries WHERE reader_id=$1 AND book_id=$2)`, *readerID, id).Scan(&detail.Liked, &detail.InBookshelf); err != nil {
		return BookDetail{}, err
	}
	var history History
	err = service.db.QueryRowContext(ctx, `SELECT id,book_id,chapter_id,chapter_no,position_type,position_value,progress_percent,last_read_at
		FROM reader_reading_history WHERE reader_id=$1 AND book_id=$2`, *readerID, id).
		Scan(&history.ID, &history.BookID, &history.ChapterID, &history.ChapterNo, &history.PositionType, &history.PositionValue, &history.ProgressPercent, &history.LastReadAt)
	if errors.Is(err, sql.ErrNoRows) {
		return detail, nil
	}
	if err != nil {
		return BookDetail{}, err
	}
	chapter, err := service.chapters.Chapter(ctx, history.ChapterID)
	if errors.Is(err, novelcontract.ErrChapterNotFound) {
		return detail, nil
	}
	if err != nil {
		return BookDetail{}, err
	}
	if chapter.BookID != history.BookID {
		return detail, nil
	}
	history.ChapterName = chapter.Name
	detail.History = &history
	return detail, nil
}

func (service *Service) BookStatus(ctx context.Context, book Book, readerID *int64) (commercecontract.AccessResult, error) {
	if service.access == nil {
		return commercecontract.AccessResult{}, commercecontract.ErrUnavailable
	}
	results, err := service.access.AccessReaders(ctx, []commercecontract.AccessRequest{{ReaderID: readerID, BookID: book.ID, ChargeMode: book.ChargeMode, FixedPriceCoin: book.FixedPriceCoin}})
	if err != nil {
		return commercecontract.AccessResult{}, err
	}
	if len(results) != 1 {
		return commercecontract.AccessResult{}, commercecontract.ErrUnavailable
	}
	return normalizeBookStatus(results[0]), nil
}

func (service *Service) BookStatuses(ctx context.Context, books []Book, readerID *int64) (map[int64]commercecontract.AccessResult, error) {
	statuses := make(map[int64]commercecontract.AccessResult, len(books))
	if len(books) == 0 {
		return statuses, nil
	}
	if service.access == nil {
		return nil, commercecontract.ErrUnavailable
	}
	requests := make([]commercecontract.AccessRequest, 0, len(books))
	for _, book := range books {
		requests = append(requests, commercecontract.AccessRequest{ReaderID: readerID, BookID: book.ID, ChargeMode: book.ChargeMode, FixedPriceCoin: book.FixedPriceCoin})
	}
	results, err := service.access.AccessReaders(ctx, requests)
	if err != nil {
		return nil, err
	}
	if len(results) != len(books) {
		return nil, commercecontract.ErrUnavailable
	}
	for index, result := range results {
		statuses[books[index].ID] = normalizeBookStatus(result)
	}
	return statuses, nil
}

func normalizeBookStatus(status commercecontract.AccessResult) commercecontract.AccessResult {
	switch status.AccessReason {
	case "book_owned", "membership", "login_required", "unsupported_mode":
		return status
	}
	switch status.ChargeMode {
	case "login_free":
		status.Readable, status.AccessReason = true, "login_free"
	case "membership_only":
		status.Readable, status.AccessReason = false, "membership_required"
	case "fixed_price":
		status.Readable, status.AccessReason = false, "book_purchase_required"
	case "word_charge":
		status.Readable, status.AccessReason, status.Purchasable = false, "chapter_purchase_required", false
	default:
		status.Readable, status.AccessReason = false, "unsupported_mode"
	}
	return status
}

func (service *Service) Chapters(ctx context.Context, bookID int64, readerID *int64) ([]Chapter, error) {
	if service == nil || service.chapters == nil {
		return nil, novelcontract.ErrUnavailable
	}
	source, err := service.chapters.Chapters(ctx, bookID)
	if err != nil {
		return nil, err
	}
	chapters := make([]Chapter, 0, len(source))
	for _, value := range source {
		chapters = append(chapters, chapterFromContract(value))
	}
	if service.access == nil || len(chapters) == 0 {
		return chapters, nil
	}
	requests := make([]commercecontract.AccessRequest, 0, len(chapters))
	for _, chapter := range chapters {
		chapterID := chapter.ID
		requests = append(requests, commercecontract.AccessRequest{ReaderID: readerID, BookID: chapter.BookID, ChapterID: chapterID, ChargeMode: chapter.ChargeMode, ChapterWordCount: chapter.WordCount})
	}
	results, err := service.access.AccessReaders(ctx, requests)
	if err != nil {
		return nil, err
	}
	if len(results) != len(chapters) {
		return nil, commercecontract.ErrUnavailable
	}
	for index := range results {
		chapters[index].Access = results[index]
	}
	return chapters, nil
}

func (service *Service) Chapter(ctx context.Context, id int64, readerID *int64) (Chapter, string, ChapterContentMeta, error) {
	if service == nil || service.chapters == nil {
		return Chapter{}, "", ChapterContentMeta{}, novelcontract.ErrUnavailable
	}
	metadata, err := service.chapters.Chapter(ctx, id)
	if errors.Is(err, novelcontract.ErrChapterNotFound) {
		return Chapter{}, "", ChapterContentMeta{}, notFound("章节不存在")
	}
	if err != nil {
		return Chapter{}, "", ChapterContentMeta{}, err
	}
	chapter := chapterFromContract(metadata)
	if service.access != nil {
		chapterID := chapter.ID
		results, accessErr := service.access.AccessReaders(ctx, []commercecontract.AccessRequest{{ReaderID: readerID, BookID: chapter.BookID, ChapterID: chapterID, ChargeMode: chapter.ChargeMode, ChapterWordCount: chapter.WordCount}})
		if accessErr != nil {
			return chapter, "", ChapterContentMeta{}, accessErr
		}
		if len(results) != 1 {
			return chapter, "", ChapterContentMeta{}, commercecontract.ErrUnavailable
		}
		if !results[0].Readable {
			return chapter, "", ChapterContentMeta{}, accessError(results[0].AccessReason)
		}
	}
	content, err := service.chapters.ChapterContent(ctx, id)
	if errors.Is(err, novelcontract.ErrObjectUnavailable) || errors.Is(err, novelcontract.ErrObjectIntegrity) {
		return chapter, "", ChapterContentMeta{}, apperror.Wrap(err, apperror.CodeUnavailable, http.StatusServiceUnavailable, "读取章节正文失败")
	}
	if errors.Is(err, novelcontract.ErrObjectEncoding) {
		return chapter, "", ChapterContentMeta{}, apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "章节正文编码无效")
	}
	if errors.Is(err, novelcontract.ErrChapterNotFound) {
		return chapter, "", ChapterContentMeta{}, notFound("章节不存在")
	}
	if err != nil {
		return chapter, "", ChapterContentMeta{}, err
	}
	chapter = chapterFromContract(content.Chapter)
	return chapter, content.Text, ChapterContentMeta{Version: content.Version, SHA256: content.SHA256, ByteSize: content.Bytes}, nil
}

func chapterFromContract(value novelcontract.Chapter) Chapter {
	return Chapter{ID: value.ID, BookID: value.BookID, No: value.Number, Name: value.Name, WordCount: value.WordCount, UpdatedAt: value.UpdatedAt, IsVIP: value.VIP, Price: value.PriceCoin, ChargeMode: value.ChargeMode, BookName: value.BookName, PrevID: value.PreviousID, NextID: value.NextID}
}

func accessError(reason string) error {
	code, message := 46102, "仅限会员阅读"
	switch reason {
	case "login_required":
		code, message = 46101, "请先登录后阅读"
	case "book_purchase_required":
		code, message = 46103, "请先购买作品"
	case "chapter_purchase_required":
		code, message = 46104, "请先购买章节"
	case "unsupported_mode":
		code, message = 46105, "作品收费模式不可用"
	}
	return apperror.New(apperror.CodeForbidden, code, message)
}

func (service *Service) SEO(ctx context.Context) (novelcontract.SEOConfig, error) {
	if service == nil || service.seo == nil {
		return novelcontract.SEOConfig{}, novelcontract.ErrUnavailable
	}
	return service.seo.SEO(ctx)
}

func (service *Service) Robots(ctx context.Context) (string, error) {
	if service == nil || service.seo == nil {
		return "", novelcontract.ErrUnavailable
	}
	return service.seo.Robots(ctx)
}

func (service *Service) Sitemap(ctx context.Context, page int) (string, error) {
	if service == nil || service.seo == nil {
		return "", novelcontract.ErrUnavailable
	}
	value, err := service.seo.Sitemap(ctx, page)
	if errors.Is(err, novelcontract.ErrBookNotFound) {
		return "", sql.ErrNoRows
	}
	return value, err
}
