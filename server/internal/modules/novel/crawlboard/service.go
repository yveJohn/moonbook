package crawlboard

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }
func (s *Service) List(ctx context.Context, keyword, sourceID, enabled string, page, size int) ([]Board, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return s.Repo.List(ctx, keyword, sourceID, enabled, page, size)
}
func (s *Service) Create(ctx context.Context, in Input) (Board, error) {
	in = normalize(in)
	if err := valid(in); err != nil {
		return Board{}, err
	}
	return s.Repo.Create(ctx, in)
}
func (s *Service) Update(ctx context.Context, id int64, in Input) (Board, error) {
	in = normalize(in)
	if id <= 0 {
		return Board{}, errors.New("invalid board id")
	}
	if err := valid(in); err != nil {
		return Board{}, err
	}
	return s.Repo.Update(ctx, id, in)
}
func (s *Service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("invalid board id")
	}
	return s.Repo.Delete(ctx, id)
}
func normalize(in Input) Input {
	in.SourceID = strings.TrimSpace(in.SourceID)
	in.BoardName = strings.TrimSpace(in.BoardName)
	in.BoardURL = strings.TrimSpace(in.BoardURL)
	in.BoardURLTemplate = strings.TrimSpace(in.BoardURLTemplate)
	in.FollowIntervalMinutes = strings.TrimSpace(in.FollowIntervalMinutes)
	in.FollowImportLimit = strings.TrimSpace(in.FollowImportLimit)
	in.SortOrder = strings.TrimSpace(in.SortOrder)
	in.Remark = strings.TrimSpace(in.Remark)
	return in
}
func valid(in Input) error {
	sid, err := strconv.ParseInt(in.SourceID, 10, 64)
	if err != nil || sid <= 0 {
		return errors.New("invalid source id")
	}
	if in.BoardName == "" || len([]rune(in.BoardName)) > 100 {
		return errors.New("invalid board name")
	}
	for _, raw := range []string{in.BoardURL, in.BoardURLTemplate} {
		if raw == "" {
			continue
		}
		u, e := url.Parse(raw)
		if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Fragment != "" {
			return errors.New("invalid board url")
		}
	}
	if in.BoardURLTemplate != "" && !strings.Contains(in.BoardURLTemplate, "{page}") {
		return errors.New("board url template must contain {page}")
	}
	iv, e := strconv.Atoi(in.FollowIntervalMinutes)
	if e != nil || iv < 1 || iv > 10080 {
		return errors.New("invalid follow interval")
	}
	lim, e := strconv.Atoi(in.FollowImportLimit)
	if e != nil || lim < 1 || lim > 1000 {
		return errors.New("invalid follow import limit")
	}
	ord, e := strconv.Atoi(in.SortOrder)
	if e != nil || ord < 0 {
		return errors.New("invalid sort order")
	}
	if len([]rune(in.Remark)) > 500 {
		return errors.New("remark too long")
	}
	return nil
}
