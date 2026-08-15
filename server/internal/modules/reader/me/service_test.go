package me

import (
	"context"
	"errors"
	"testing"
	"time"

	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
)

type fakeRepo struct {
	pref                       *Preference
	input                      PreferenceInput
	feedbackPage, feedbackSize int
	historyInput               HistoryInput
	likeResult                 BookLike
	likeErr                    error
	unlikeResult               BookLike
	unlikeErr                  error
}

type fakeDisplay struct {
	calls int
	batch novelcontract.DisplayBatch
	fn    func(novelcontract.DisplayRequest) novelcontract.DisplayBatch
}

func (display *fakeDisplay) BatchDisplay(_ context.Context, request novelcontract.DisplayRequest) (novelcontract.DisplayBatch, error) {
	display.calls++
	if display.fn != nil {
		return display.fn(request), nil
	}
	if display.batch.Books != nil || display.batch.Chapters != nil {
		return display.batch, nil
	}
	batch := novelcontract.DisplayBatch{Books: make([]novelcontract.BookDisplay, len(request.BookIDs)), Chapters: make([]novelcontract.ChapterDisplay, len(request.ChapterIDs))}
	for index, id := range request.BookIDs {
		batch.Books[index] = novelcontract.BookDisplay{ID: id, Name: "测试作品", Author: "测试作者", Published: true, Found: true}
	}
	for index, id := range request.ChapterIDs {
		batch.Chapters[index] = novelcontract.ChapterDisplay{ID: id, BookID: 2, Number: 2, Name: "测试章节", Enabled: true, Found: true}
	}
	return batch, nil
}

func (f *fakeRepo) ListBookshelf(context.Context, int64) ([]Bookshelf, error) { return nil, nil }
func (f *fakeRepo) GetBookshelf(context.Context, int64, int64) (*Bookshelf, error) {
	return nil, nil
}
func (f *fakeRepo) AddBookshelf(context.Context, int64, int64) (Bookshelf, error) {
	return Bookshelf{ID: 9223372036854775807, BookID: 9223372036854775806}, nil
}
func (f *fakeRepo) RemoveBookshelf(context.Context, int64, int64) (bool, error) { return true, nil }
func (f *fakeRepo) ListLikes(context.Context, int64) ([]LikedBook, error)       { return nil, nil }
func (f *fakeRepo) Like(context.Context, int64, int64) (BookLike, error) {
	return f.likeResult, f.likeErr
}
func (f *fakeRepo) Unlike(context.Context, int64, int64) (BookLike, error) {
	return f.unlikeResult, f.unlikeErr
}
func (f *fakeRepo) CreateFeedback(context.Context, int64, string) (Feedback, error) {
	return Feedback{ID: 1, CreatedAt: time.Now()}, nil
}
func (f *fakeRepo) ListFeedbacks(_ context.Context, _ int64, p, s int) ([]Feedback, int64, error) {
	f.feedbackPage = p
	f.feedbackSize = s
	return []Feedback{}, 0, nil
}
func (f *fakeRepo) ListHistory(context.Context, int64) ([]History, error)      { return nil, nil }
func (f *fakeRepo) GetHistory(context.Context, int64, int64) (*History, error) { return nil, nil }
func (f *fakeRepo) UpdateHistory(_ context.Context, _ int64, _ int64, in HistoryInput) (History, error) {
	f.historyInput = in
	return History{ID: 1}, nil
}
func (f *fakeRepo) GetPreference(context.Context, int64) (*Preference, error) { return f.pref, nil }
func (f *fakeRepo) UpdatePreference(_ context.Context, _ int64, in PreferenceInput) (Preference, error) {
	f.input = in
	return Preference{ID: 2, FontSize: *in.FontSize, LineHeight: in.LineHeight, Theme: in.Theme, ReadingMode: in.ReadingMode}, nil
}

type immediateTransactor struct {
	err error
}

func (tx immediateTransactor) Within(ctx context.Context, fn func(context.Context) error) error {
	if tx.err != nil {
		return tx.err
	}
	return fn(ctx)
}

type fakeLikes struct {
	lockErr       error
	summaryErr    error
	lockedBookID  int64
	summaryBookID int64
	summaryCount  int64
}

func (likes *fakeLikes) LockPublishedBook(_ context.Context, bookID int64) error {
	likes.lockedBookID = bookID
	return likes.lockErr
}

func (likes *fakeLikes) SetLikeCount(_ context.Context, bookID, count int64) error {
	likes.summaryBookID = bookID
	likes.summaryCount = count
	return likes.summaryErr
}

func newTestService(repo Repository, display novelcontract.DisplayReader) *Service {
	likes := &fakeLikes{}
	return NewService(repo, display, immediateTransactor{}, likes, likes)
}

func TestParseIDKeepsMaxInt64(t *testing.T) {
	v, e := ParseID("9223372036854775807")
	if e != nil || v != 9223372036854775807 {
		t.Fatalf("id=%d err=%v", v, e)
	}
	if _, e = ParseID("9007199254740993"); e != nil {
		t.Fatal(e)
	}
	if _, e = ParseID("0"); e == nil {
		t.Fatal("zero must fail")
	}
}
func TestPreferenceDefaultsAndNormalization(t *testing.T) {
	f := &fakeRepo{}
	s := newTestService(f, &fakeDisplay{})
	v, e := s.GetPreference(context.Background(), 1)
	if e != nil || v.FontSize != 20 || v.LineHeight != "1.80" || v.Theme != "cream" || v.ReadingMode != "page" {
		t.Fatalf("default=%+v err=%v", v, e)
	}
	v, e = s.UpdatePreference(context.Background(), 1, PreferenceInput{FontSize: ptr(99), LineHeight: "1.401", Theme: "bad", ReadingMode: "bad"})
	if e != nil || v.FontSize != 28 || v.LineHeight != "1.40" || v.Theme != "cream" || v.ReadingMode != "page" {
		t.Fatalf("normalized=%+v err=%v", v, e)
	}
}
func TestHistoryRejectsInvalidProgressAndDefaultsMode(t *testing.T) {
	f := &fakeRepo{}
	s := newTestService(f, &fakeDisplay{})
	_, e := s.UpdateHistory(context.Background(), 1, 2, HistoryInput{ChapterID: 3, PositionType: "scroll", ProgressPercent: "100.01"})
	if e == nil {
		t.Fatal("out-of-range progress accepted")
	}
	_, e = s.UpdateHistory(context.Background(), 1, 2, HistoryInput{ChapterID: 3, PositionType: "page", PositionValue: 4, ProgressPercent: "12.5"})
	if e != nil || f.historyInput.ProgressPercent != "12.5" {
		t.Fatalf("history=%+v err=%v", f.historyInput, e)
	}
}
func TestFeedbackPaginationBounds(t *testing.T) {
	f := &fakeRepo{}
	s := newTestService(f, &fakeDisplay{})
	_, _, e := s.ListFeedbacks(context.Background(), 1, 0, 1000)
	if e != nil || f.feedbackPage != 1 || f.feedbackSize != 100 {
		t.Fatalf("page=%d size=%d err=%v", f.feedbackPage, f.feedbackSize, e)
	}
}

func TestPersonalContentUsesOneDisplayBatchAndFiltersUnavailableTargets(t *testing.T) {
	chapterID := int64(31)
	repo := &fakeRepoWithContent{
		fakeRepo: fakeRepo{},
		shelf:    []Bookshelf{{ID: 1, BookID: 11, LastChapterID: &chapterID}, {ID: 2, BookID: 12}},
		likes:    []LikedBook{{BookLikeID: 3, BookID: 11}, {BookLikeID: 4, BookID: 12}},
		history:  []History{{ID: 5, BookID: 11, ChapterID: 31}, {ID: 6, BookID: 12, ChapterID: 32}},
	}
	display := &fakeDisplay{batch: novelcontract.DisplayBatch{
		Books: []novelcontract.BookDisplay{
			{ID: 11, Name: "可见作品", Author: "作者", Found: true, Published: true},
			{ID: 12, Name: "草稿作品", Found: true, Published: false},
		},
		Chapters: []novelcontract.ChapterDisplay{
			{ID: 31, BookID: 11, Name: "末章", Found: true, Enabled: true},
			{ID: 32, BookID: 12, Name: "草稿章节", Found: true, Enabled: true},
		},
	}}
	items, err := newTestService(repo, display).ListBookshelf(context.Background(), 1)
	if err != nil || len(items) != 1 || items[0].BookName != "可见作品" || items[0].LastChapterName == nil || *items[0].LastChapterName != "末章" {
		t.Fatalf("shelf=%+v err=%v", items, err)
	}
	if display.calls != 1 {
		t.Fatalf("display calls=%d, want 1", display.calls)
	}
	likes, err := newTestService(repo, display).ListLikes(context.Background(), 1)
	if err != nil || len(likes) != 1 || likes[0].BookID != 11 || display.calls != 2 {
		t.Fatalf("likes=%+v calls=%d err=%v", likes, display.calls, err)
	}
	history, err := newTestService(repo, display).ListHistory(context.Background(), 1)
	if err != nil || len(history) != 1 || history[0].ChapterName != "末章" || display.calls != 3 {
		t.Fatalf("history=%+v calls=%d err=%v", history, display.calls, err)
	}
}

type fakeRepoWithContent struct {
	fakeRepo
	shelf   []Bookshelf
	likes   []LikedBook
	history []History
}

func (repo *fakeRepoWithContent) ListBookshelf(context.Context, int64) ([]Bookshelf, error) {
	return repo.shelf, nil
}

func (repo *fakeRepoWithContent) ListLikes(context.Context, int64) ([]LikedBook, error) {
	return repo.likes, nil
}

func (repo *fakeRepoWithContent) ListHistory(context.Context, int64) ([]History, error) {
	return repo.history, nil
}

func TestDisplayCallCountDoesNotGrowWithPageSize(t *testing.T) {
	likes := make([]LikedBook, 250)
	for index := range likes {
		likes[index] = LikedBook{BookLikeID: int64(index + 1), BookID: int64(index + 1000)}
	}
	repo := &fakeRepoWithContent{likes: likes}
	var requested int
	display := &fakeDisplay{fn: func(request novelcontract.DisplayRequest) novelcontract.DisplayBatch {
		requested = len(request.BookIDs)
		books := make([]novelcontract.BookDisplay, len(request.BookIDs))
		for index, id := range request.BookIDs {
			books[index] = novelcontract.BookDisplay{ID: id, Found: true, Published: true}
		}
		return novelcontract.DisplayBatch{Books: books, Chapters: []novelcontract.ChapterDisplay{}}
	}}
	items, err := newTestService(repo, display).ListLikes(context.Background(), 1)
	if err != nil || len(items) != len(likes) {
		t.Fatalf("items=%d err=%v", len(items), err)
	}
	if display.calls != 1 || requested != len(likes) {
		t.Fatalf("display calls=%d requested=%d", display.calls, requested)
	}
}

func TestLikeTransactionOrchestrationAndFailures(t *testing.T) {
	forced := errors.New("forced failure")
	tests := []struct {
		name       string
		repo       *fakeRepo
		likes      *fakeLikes
		tx         immediateTransactor
		wantResult BookLike
	}{
		{name: "like", repo: &fakeRepo{likeResult: BookLike{ID: 7, BookID: 11, Liked: true, LikeCount: 3}}, likes: &fakeLikes{}, wantResult: BookLike{ID: 7, BookID: 11, Liked: true, LikeCount: 3}},
		{name: "unlike", repo: &fakeRepo{unlikeResult: BookLike{BookID: 11, LikeCount: 2}}, likes: &fakeLikes{}, wantResult: BookLike{BookID: 11, LikeCount: 2}},
		{name: "book failure", repo: &fakeRepo{likeResult: BookLike{ID: 7}}, likes: &fakeLikes{lockErr: forced}},
		{name: "reader failure", repo: &fakeRepo{likeErr: forced}, likes: &fakeLikes{}},
		{name: "summary failure", repo: &fakeRepo{likeResult: BookLike{ID: 7, BookID: 11, Liked: true, LikeCount: 3}}, likes: &fakeLikes{summaryErr: forced}},
		{name: "transaction failure", repo: &fakeRepo{}, likes: &fakeLikes{}, tx: immediateTransactor{err: forced}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(test.repo, &fakeDisplay{}, test.tx, test.likes, test.likes)
			var result BookLike
			var err error
			if test.name == "unlike" {
				result, err = service.Unlike(context.Background(), 5, 11)
			} else {
				result, err = service.Like(context.Background(), 5, 11)
			}
			if test.wantResult.BookID != 0 {
				if err != nil || result != test.wantResult || test.likes.lockedBookID != 11 || test.likes.summaryBookID != 11 || test.likes.summaryCount != int64(test.wantResult.LikeCount) {
					t.Fatalf("result=%+v likes=%+v err=%v", result, test.likes, err)
				}
				return
			}
			if !errors.Is(err, ErrBookUnavailable) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
func ptr(v int) *int { return &v }
