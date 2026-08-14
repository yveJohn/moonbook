package me

import (
	"context"
	"testing"
	"time"
)

type fakeRepo struct {
	pref                       *Preference
	input                      PreferenceInput
	feedbackPage, feedbackSize int
	historyInput               HistoryInput
}

func (f *fakeRepo) ListBookshelf(context.Context, int64) ([]Bookshelf, error) { return nil, nil }
func (f *fakeRepo) AddBookshelf(context.Context, int64, int64) (Bookshelf, error) {
	return Bookshelf{ID: 9223372036854775807, BookID: 9223372036854775806}, nil
}
func (f *fakeRepo) RemoveBookshelf(context.Context, int64, int64) (bool, error) { return true, nil }
func (f *fakeRepo) ListLikes(context.Context, int64) ([]LikedBook, error)       { return nil, nil }
func (f *fakeRepo) Like(context.Context, int64, int64) (BookLike, error)        { return BookLike{}, nil }
func (f *fakeRepo) Unlike(context.Context, int64, int64) (BookLike, error)      { return BookLike{}, nil }
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
	s := NewService(f)
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
	s := NewService(f)
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
	s := NewService(f)
	_, _, e := s.ListFeedbacks(context.Background(), 1, 0, 1000)
	if e != nil || f.feedbackPage != 1 || f.feedbackSize != 100 {
		t.Fatalf("page=%d size=%d err=%v", f.feedbackPage, f.feedbackSize, e)
	}
}
func ptr(v int) *int { return &v }
