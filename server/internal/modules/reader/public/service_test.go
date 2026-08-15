package public

import (
	"context"
	"errors"
	"testing"

	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

type publicNovelStub struct {
	books        []novelcontract.Book
	chapters     []novelcontract.Chapter
	chapter      novelcontract.Chapter
	content      novelcontract.ChapterContent
	contentCalls int
}

func (stub *publicNovelStub) FeaturedBooks(context.Context) ([]novelcontract.Book, error) {
	return stub.books, nil
}
func (stub *publicNovelStub) RandomBooks(context.Context) ([]novelcontract.Book, error) {
	return stub.books, nil
}
func (stub *publicNovelStub) Books(context.Context, string, string, string, int, int) ([]novelcontract.Book, int64, error) {
	return stub.books, int64(len(stub.books)), nil
}
func (stub *publicNovelStub) Categories(context.Context, string) ([]novelcontract.Category, error) {
	return []novelcontract.Category{}, nil
}
func (stub *publicNovelStub) Book(context.Context, int64) (novelcontract.Book, error) {
	if len(stub.books) == 0 {
		return novelcontract.Book{}, novelcontract.ErrBookNotFound
	}
	return stub.books[0], nil
}
func (stub *publicNovelStub) Chapters(context.Context, int64) ([]novelcontract.Chapter, error) {
	return stub.chapters, nil
}
func (stub *publicNovelStub) Chapter(context.Context, int64) (novelcontract.Chapter, error) {
	return stub.chapter, nil
}
func (stub *publicNovelStub) ChapterContent(context.Context, int64) (novelcontract.ChapterContent, error) {
	stub.contentCalls++
	return stub.content, nil
}
func (*publicNovelStub) SEO(context.Context) (novelcontract.SEOConfig, error) {
	return novelcontract.SEOConfig{}, nil
}
func (*publicNovelStub) Robots(context.Context) (string, error)       { return "robots", nil }
func (*publicNovelStub) Sitemap(context.Context, int) (string, error) { return "sitemap", nil }

type publicAccessStub struct {
	results []commercecontract.AccessResult
	err     error
}

func (stub publicAccessStub) AccessReaders(context.Context, []commercecontract.AccessRequest) ([]commercecontract.AccessResult, error) {
	return stub.results, stub.err
}

func TestChapterChecksAccessBeforeReadingContent(t *testing.T) {
	novel := &publicNovelStub{chapter: novelcontract.Chapter{ID: 9, BookID: 8, ChargeMode: "fixed_price"}}
	service := NewService(nil, novel, novel, novel, publicAccessStub{results: []commercecontract.AccessResult{{BookID: 8, AccessReason: "book_purchase_required"}}})

	_, _, _, err := service.Chapter(context.Background(), 9, nil)
	public := apperror.Expose(err)
	if public.HTTPStatus != 46103 || novel.contentCalls != 0 {
		t.Fatalf("error=%+v contentCalls=%d", public, novel.contentCalls)
	}
}

func TestChapterReturnsContractContentAndNeighbors(t *testing.T) {
	previous, next := int64(7), int64(10)
	chapter := novelcontract.Chapter{ID: 9, BookID: 8, Number: 2, PreviousID: &previous, NextID: &next, ChargeMode: "login_free"}
	novel := &publicNovelStub{chapter: chapter, content: novelcontract.ChapterContent{Chapter: chapter, Text: "正文", Version: 3, SHA256: "hash", Bytes: 6}}
	service := NewService(nil, novel, novel, novel, publicAccessStub{results: []commercecontract.AccessResult{{BookID: 8, Readable: true}}})

	got, text, metadata, err := service.Chapter(context.Background(), 9, nil)
	if err != nil || text != "正文" || metadata.Version != 3 || metadata.SHA256 != "hash" || metadata.ByteSize != 6 || got.PrevID == nil || *got.PrevID != previous || got.NextID == nil || *got.NextID != next || novel.contentCalls != 1 {
		t.Fatalf("chapter=%+v text=%q metadata=%+v calls=%d err=%v", got, text, metadata, novel.contentCalls, err)
	}
}

func TestBookStatusesRejectsIncompleteCommerceBatch(t *testing.T) {
	novel := &publicNovelStub{}
	service := NewService(nil, novel, novel, novel, publicAccessStub{results: []commercecontract.AccessResult{}})
	_, err := service.BookStatuses(context.Background(), []Book{{ID: 1}, {ID: 2}}, nil)
	if !errors.Is(err, commercecontract.ErrUnavailable) {
		t.Fatalf("error=%v", err)
	}
}
