package me

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/utils/logger"
)

var (
	ErrInvalidID          = apperror.New(apperror.CodeInvalidArgument, 200, "ID必须是正整数字符串")
	ErrBookUnavailable    = apperror.New(apperror.CodeNotFound, 200, "作品不存在或暂不可见")
	ErrChapterUnavailable = apperror.New(apperror.CodeNotFound, 200, "章节不存在或暂不可见")
	ErrInvalidInput       = apperror.New(apperror.CodeInvalidArgument, 200, "请求参数不合法")
	ErrMeUnavailable      = apperror.New(apperror.CodeInternal, 500, "个人数据服务暂不可用")
)

type Service struct {
	repo        Repository
	display     novelcontract.DisplayReader
	tx          Transactor
	likeLocker  novelcontract.LikeBookLocker
	likeSummary novelcontract.LikeSummaryWriter
	notifier    FeedbackNotifier
}

func NewService(repo Repository, display novelcontract.DisplayReader, transactor Transactor, likeLocker novelcontract.LikeBookLocker, likeSummary novelcontract.LikeSummaryWriter) *Service {
	return &Service{repo: repo, display: display, tx: transactor, likeLocker: likeLocker, likeSummary: likeSummary}
}

func (s *Service) SetFeedbackNotifier(notifier FeedbackNotifier) {
	s.notifier = notifier
}
func (s *Service) ListBookshelf(ctx context.Context, id int64) ([]Bookshelf, error) {
	items, err := s.repo.ListBookshelf(ctx, id)
	if err != nil {
		return nil, err
	}
	books, chapters, err := s.displays(ctx, bookshelfDisplayIDs(items))
	if err != nil {
		return nil, err
	}
	out := make([]Bookshelf, 0, len(items))
	for _, item := range items {
		book := books[item.BookID]
		if !book.Found || !book.Published {
			continue
		}
		item.BookName = book.Name
		item.AuthorName = book.Author
		if item.LastChapterID != nil {
			chapter := chapters[*item.LastChapterID]
			if chapter.Found && chapter.Enabled && chapter.BookID == item.BookID {
				name := chapter.Name
				item.LastChapterName = &name
			}
		}
		out = append(out, item)
	}
	return out, nil
}
func (s *Service) AddBookshelf(ctx context.Context, id, b int64) (Bookshelf, error) {
	existing, err := s.repo.GetBookshelf(ctx, id, b)
	if err != nil {
		return Bookshelf{}, err
	}
	request := novelcontract.DisplayRequest{BookIDs: []int64{b}}
	if existing != nil && existing.LastChapterID != nil {
		request.ChapterIDs = []int64{*existing.LastChapterID}
	}
	books, chapters, err := s.displays(ctx, request)
	if err != nil {
		return Bookshelf{}, err
	}
	book := books[b]
	if !book.Found || !book.Published {
		return Bookshelf{}, ErrBookUnavailable
	}
	value, err := s.repo.AddBookshelf(ctx, id, b)
	if errors.Is(err, context.Canceled) {
		return value, err
	}
	if err != nil {
		return value, ErrBookUnavailable
	}
	value.BookName = book.Name
	value.AuthorName = book.Author
	if value.LastChapterID != nil {
		chapter := chapters[*value.LastChapterID]
		if chapter.Found && chapter.Enabled && chapter.BookID == value.BookID {
			name := chapter.Name
			value.LastChapterName = &name
		}
	}
	return value, nil
}
func (s *Service) RemoveBookshelf(ctx context.Context, id, b int64) (bool, error) {
	v, e := s.repo.RemoveBookshelf(ctx, id, b)
	return v, e
}
func (s *Service) ListLikes(ctx context.Context, id int64) ([]LikedBook, error) {
	items, err := s.repo.ListLikes(ctx, id)
	if err != nil {
		return nil, err
	}
	request := novelcontract.DisplayRequest{BookIDs: make([]int64, 0, len(items))}
	seenBooks := make(map[int64]struct{}, len(items))
	for _, item := range items {
		request.BookIDs = appendUniqueID(request.BookIDs, seenBooks, item.BookID)
	}
	books, _, err := s.displays(ctx, request)
	if err != nil {
		return nil, err
	}
	out := make([]LikedBook, 0, len(items))
	for _, item := range items {
		book := books[item.BookID]
		if !book.Found || !book.Published {
			continue
		}
		item.BookName = book.Name
		item.AuthorName = book.Author
		item.Description = book.Description
		item.CategoryCode = book.CategoryCode
		item.CategoryName = book.CategoryName
		item.WordCount = book.WordCount
		item.LikeCount = book.LikeCount
		out = append(out, item)
	}
	return out, nil
}
func (s *Service) Like(ctx context.Context, id, b int64) (BookLike, error) {
	return s.setLike(ctx, id, b, true)
}
func (s *Service) Unlike(ctx context.Context, id, b int64) (BookLike, error) {
	return s.setLike(ctx, id, b, false)
}

func (s *Service) setLike(ctx context.Context, readerID, bookID int64, liked bool) (BookLike, error) {
	if s == nil || s.repo == nil || s.tx == nil || s.likeLocker == nil || s.likeSummary == nil {
		return BookLike{}, ErrMeUnavailable
	}
	var result BookLike
	err := s.tx.Within(ctx, func(txCtx context.Context) error {
		if err := s.likeLocker.LockPublishedBook(txCtx, bookID); err != nil {
			return err
		}
		var err error
		if liked {
			result, err = s.repo.Like(txCtx, readerID, bookID)
		} else {
			result, err = s.repo.Unlike(txCtx, readerID, bookID)
		}
		if err != nil {
			return err
		}
		return s.likeSummary.SetLikeCount(txCtx, bookID, int64(result.LikeCount))
	})
	if err != nil {
		return BookLike{}, ErrBookUnavailable
	}
	return result, nil
}
func (s *Service) CreateFeedback(ctx context.Context, id int64, content string) (Feedback, error) {
	content = strings.TrimSpace(content)
	if content == "" || len([]rune(content)) > 5000 {
		return Feedback{}, ErrInvalidInput
	}
	feedback, err := s.repo.CreateFeedback(ctx, id, content)
	if err != nil {
		return Feedback{}, err
	}
	if s.notifier != nil {
		if notifyErr := s.notifier.NotifyFeedback(ctx, feedback.ID, id, feedback.CreatedAt, feedback.Content); notifyErr != nil {
			logger.WithCtx(ctx).Mod("serverchan").
				Field("feedback_id", strconv.FormatInt(feedback.ID, 10)).
				Field("reader_id", strconv.FormatInt(id, 10)).
				Err(notifyErr).
				Warn("用户反馈通知发送失败")
		}
	}
	return feedback, nil
}
func (s *Service) ListFeedbacks(ctx context.Context, id int64, page, size int) ([]Feedback, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}
	return s.repo.ListFeedbacks(ctx, id, page, size)
}
func (s *Service) ListHistory(ctx context.Context, id int64) ([]History, error) {
	items, err := s.repo.ListHistory(ctx, id)
	if err != nil {
		return nil, err
	}
	books, chapters, err := s.displays(ctx, historyDisplayIDs(items))
	if err != nil {
		return nil, err
	}
	out := make([]History, 0, len(items))
	for _, item := range items {
		book, chapter := books[item.BookID], chapters[item.ChapterID]
		if !book.Found || !book.Published || !chapter.Found || !chapter.Enabled || chapter.BookID != item.BookID {
			continue
		}
		item.ChapterName = chapter.Name
		out = append(out, item)
	}
	return out, nil
}
func (s *Service) GetHistory(ctx context.Context, id, b int64) (*History, error) {
	item, err := s.repo.GetHistory(ctx, id, b)
	if err != nil || item == nil {
		return item, err
	}
	books, chapters, err := s.displays(ctx, novelcontract.DisplayRequest{BookIDs: []int64{item.BookID}, ChapterIDs: []int64{item.ChapterID}})
	if err != nil {
		return nil, err
	}
	book, chapter := books[item.BookID], chapters[item.ChapterID]
	if !book.Found || !book.Published || !chapter.Found || !chapter.Enabled || chapter.BookID != item.BookID {
		return nil, nil
	}
	item.ChapterName = chapter.Name
	return item, nil
}
func (s *Service) UpdateHistory(ctx context.Context, id, b int64, in HistoryInput) (History, error) {
	if in.PositionType == "" {
		in.PositionType = "scroll"
	}
	if in.ChapterID <= 0 || in.PositionValue < 0 || in.PositionType != "scroll" && in.PositionType != "page" || in.ProgressPercent == "" {
		return History{}, ErrInvalidInput
	}
	p, e := strconv.ParseFloat(in.ProgressPercent, 64)
	if e != nil || p < 0 || p > 100 {
		return History{}, ErrInvalidInput
	}
	books, chapters, err := s.displays(ctx, novelcontract.DisplayRequest{BookIDs: []int64{b}, ChapterIDs: []int64{in.ChapterID}})
	if err != nil {
		return History{}, err
	}
	book, chapter := books[b], chapters[in.ChapterID]
	if !book.Found || !book.Published || !chapter.Found || !chapter.Enabled || chapter.BookID != b {
		return History{}, ErrChapterUnavailable
	}
	if in.ChapterNo == nil {
		in.ChapterNo = &chapter.Number
	}
	v, e := s.repo.UpdateHistory(ctx, id, b, in)
	if e != nil {
		return v, ErrChapterUnavailable
	}
	v.ChapterName = chapter.Name
	return v, nil
}

func (s *Service) displays(ctx context.Context, request novelcontract.DisplayRequest) (map[int64]novelcontract.BookDisplay, map[int64]novelcontract.ChapterDisplay, error) {
	if s == nil || s.display == nil {
		return nil, nil, ErrMeUnavailable
	}
	batch, err := s.display.BatchDisplay(ctx, request)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, nil, err
		}
		return nil, nil, ErrMeUnavailable
	}
	books := make(map[int64]novelcontract.BookDisplay, len(batch.Books))
	for _, book := range batch.Books {
		books[book.ID] = book
	}
	chapters := make(map[int64]novelcontract.ChapterDisplay, len(batch.Chapters))
	for _, chapter := range batch.Chapters {
		chapters[chapter.ID] = chapter
	}
	return books, chapters, nil
}

func bookshelfDisplayIDs(items []Bookshelf) novelcontract.DisplayRequest {
	request := novelcontract.DisplayRequest{BookIDs: make([]int64, 0, len(items)), ChapterIDs: make([]int64, 0, len(items))}
	seenBooks := make(map[int64]struct{}, len(items))
	seenChapters := make(map[int64]struct{}, len(items))
	for _, item := range items {
		request.BookIDs = appendUniqueID(request.BookIDs, seenBooks, item.BookID)
		if item.LastChapterID != nil {
			request.ChapterIDs = appendUniqueID(request.ChapterIDs, seenChapters, *item.LastChapterID)
		}
	}
	return request
}

func historyDisplayIDs(items []History) novelcontract.DisplayRequest {
	request := novelcontract.DisplayRequest{BookIDs: make([]int64, 0, len(items)), ChapterIDs: make([]int64, 0, len(items))}
	seenBooks := make(map[int64]struct{}, len(items))
	seenChapters := make(map[int64]struct{}, len(items))
	for _, item := range items {
		request.BookIDs = appendUniqueID(request.BookIDs, seenBooks, item.BookID)
		request.ChapterIDs = appendUniqueID(request.ChapterIDs, seenChapters, item.ChapterID)
	}
	return request
}

func appendUniqueID(ids []int64, seen map[int64]struct{}, id int64) []int64 {
	if _, exists := seen[id]; exists {
		return ids
	}
	seen[id] = struct{}{}
	return append(ids, id)
}
func (s *Service) GetPreference(ctx context.Context, id int64) (Preference, error) {
	v, e := s.repo.GetPreference(ctx, id)
	if e != nil {
		return Preference{}, e
	}
	if v == nil {
		return Preference{FontSize: 20, LineHeight: "1.80", Theme: "cream", ReadingMode: "page"}, nil
	}
	return *v, nil
}
func (s *Service) UpdatePreference(ctx context.Context, id int64, in PreferenceInput) (Preference, error) {
	fs := 20
	if in.FontSize != nil {
		fs = *in.FontSize
	}
	if fs < 14 {
		fs = 14
	}
	if fs > 28 {
		fs = 28
	}
	lh := in.LineHeight
	if lh == "" {
		lh = "1.80"
	}
	n, e := strconv.ParseFloat(lh, 64)
	if e != nil || n < 1.4 || n > 2.4 {
		return Preference{}, ErrInvalidInput
	}
	if in.Theme != "cream" && in.Theme != "night" && in.Theme != "green" {
		in.Theme = "cream"
	}
	if in.ReadingMode != "scroll" && in.ReadingMode != "page" {
		in.ReadingMode = "page"
	}
	in.FontSize = &fs
	in.LineHeight = fmt.Sprintf("%.2f", n)
	return s.repo.UpdatePreference(ctx, id, in)
}

func ParseID(raw string) (int64, error) {
	v, e := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if e != nil || v <= 0 {
		return 0, ErrInvalidID
	}
	return v, nil
}
func parseProgress(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "0", nil
	}
	var s string
	if raw[0] == '"' {
		if json.Unmarshal(raw, &s) != nil {
			return "", ErrInvalidInput
		}
	} else {
		s = string(raw)
	}
	p, e := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if e != nil || math.IsNaN(p) || p < 0 || p > 100 {
		return "", ErrInvalidInput
	}
	return fmt.Sprintf("%.2f", p), nil
}
