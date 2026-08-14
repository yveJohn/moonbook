package me

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

var (
	ErrInvalidID          = apperror.New(apperror.CodeInvalidArgument, 200, "ID必须是正整数字符串")
	ErrBookUnavailable    = apperror.New(apperror.CodeNotFound, 200, "作品不存在或暂不可见")
	ErrChapterUnavailable = apperror.New(apperror.CodeNotFound, 200, "章节不存在或暂不可见")
	ErrInvalidInput       = apperror.New(apperror.CodeInvalidArgument, 200, "请求参数不合法")
	ErrMeUnavailable      = apperror.New(apperror.CodeInternal, 500, "个人数据服务暂不可用")
)

type Service struct{ repo Repository }

func NewService(repo Repository) *Service { return &Service{repo: repo} }
func (s *Service) ListBookshelf(ctx context.Context, id int64) ([]Bookshelf, error) {
	return s.repo.ListBookshelf(ctx, id)
}
func (s *Service) AddBookshelf(ctx context.Context, id, b int64) (Bookshelf, error) {
	v, e := s.repo.AddBookshelf(ctx, id, b)
	if errors.Is(e, context.Canceled) {
		return v, e
	}
	if e != nil {
		return v, ErrBookUnavailable
	}
	return v, nil
}
func (s *Service) RemoveBookshelf(ctx context.Context, id, b int64) (bool, error) {
	v, e := s.repo.RemoveBookshelf(ctx, id, b)
	return v, e
}
func (s *Service) ListLikes(ctx context.Context, id int64) ([]LikedBook, error) {
	return s.repo.ListLikes(ctx, id)
}
func (s *Service) Like(ctx context.Context, id, b int64) (BookLike, error) {
	v, e := s.repo.Like(ctx, id, b)
	if e != nil {
		return v, ErrBookUnavailable
	}
	return v, nil
}
func (s *Service) Unlike(ctx context.Context, id, b int64) (BookLike, error) {
	v, e := s.repo.Unlike(ctx, id, b)
	if e != nil {
		return v, ErrBookUnavailable
	}
	return v, nil
}
func (s *Service) CreateFeedback(ctx context.Context, id int64, content string) (Feedback, error) {
	content = strings.TrimSpace(content)
	if content == "" || len([]rune(content)) > 5000 {
		return Feedback{}, ErrInvalidInput
	}
	return s.repo.CreateFeedback(ctx, id, content)
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
	return s.repo.ListHistory(ctx, id)
}
func (s *Service) GetHistory(ctx context.Context, id, b int64) (*History, error) {
	return s.repo.GetHistory(ctx, id, b)
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
	v, e := s.repo.UpdateHistory(ctx, id, b, in)
	if e != nil {
		return v, ErrChapterUnavailable
	}
	return v, nil
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
