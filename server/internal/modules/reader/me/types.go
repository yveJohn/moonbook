package me

import (
	"context"
	"database/sql"
	"time"
)

type Bookshelf struct {
	ID, BookID           int64
	BookName, AuthorName string
	LastChapterID        *int64
	LastChapterName      *string
	LastReadAt           *time.Time
}
type BookLike struct {
	ID, BookID int64
	Liked      bool
	LikeCount  int
}
type LikedBook struct {
	BookLikeID, BookID                                            int64
	BookName, AuthorName, Description, CategoryCode, CategoryName string
	WordCount, LikeCount                                          int
	LikedAt                                                       time.Time
}
type History struct {
	ID, BookID, ChapterID     int64
	ChapterNo                 int
	ChapterName, PositionType string
	PositionValue             int
	ProgressPercent           string
	LastReadAt                time.Time
}
type Preference struct {
	ID                 int64
	FontSize           int
	LineHeight         string
	Theme, ReadingMode string
}
type Feedback struct {
	ID              int64
	Content, Status string
	Reply           *string
	RepliedAt       *time.Time
	CreatedAt       time.Time
}
type HistoryInput struct {
	ChapterID       int64
	ChapterNo       *int
	PositionType    string
	PositionValue   int
	ProgressPercent string
}
type PreferenceInput struct {
	FontSize           *int
	LineHeight         string
	Theme, ReadingMode string
}

type Repository interface {
	ListBookshelf(context.Context, int64) ([]Bookshelf, error)
	GetBookshelf(context.Context, int64, int64) (*Bookshelf, error)
	AddBookshelf(context.Context, int64, int64) (Bookshelf, error)
	RemoveBookshelf(context.Context, int64, int64) (bool, error)
	ListLikes(context.Context, int64) ([]LikedBook, error)
	Like(context.Context, int64, int64) (BookLike, error)
	Unlike(context.Context, int64, int64) (BookLike, error)
	CreateFeedback(context.Context, int64, string) (Feedback, error)
	ListFeedbacks(context.Context, int64, int, int) ([]Feedback, int64, error)
	ListHistory(context.Context, int64) ([]History, error)
	GetHistory(context.Context, int64, int64) (*History, error)
	UpdateHistory(context.Context, int64, int64, HistoryInput) (History, error)
	GetPreference(context.Context, int64) (*Preference, error)
	UpdatePreference(context.Context, int64, PreferenceInput) (Preference, error)
}

type Transactor interface {
	Within(context.Context, func(context.Context) error) error
}

type SQLRepository struct{ DB *sql.DB }

var _ Repository = SQLRepository{}
