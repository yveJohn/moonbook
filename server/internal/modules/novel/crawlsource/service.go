package crawlsource

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }

func (s *Service) List(ctx context.Context, keyword, enabled string, page, size int) ([]Source, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return s.Repo.List(ctx, keyword, enabled, page, size)
}
func (s *Service) Create(ctx context.Context, in Input) (Source, error) {
	if in.CookieText != nil {
		return Source{}, errors.New("forum cookie plaintext is not accepted")
	}
	in = normalize(in)
	if err := valid(in); err != nil {
		return Source{}, err
	}
	return s.Repo.Create(ctx, in)
}
func (s *Service) Update(ctx context.Context, id int64, in Input) (Source, error) {
	if in.CookieText != nil {
		return Source{}, errors.New("forum cookie plaintext is not accepted")
	}
	in = normalize(in)
	if id <= 0 {
		return Source{}, errors.New("invalid source id")
	}
	if err := valid(in); err != nil {
		return Source{}, err
	}
	return s.Repo.Update(ctx, id, in)
}
func (s *Service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("invalid source id")
	}
	return s.Repo.Delete(ctx, id)
}

func valid(in Input) error {
	in.SourceName = strings.TrimSpace(in.SourceName)
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	if in.SourceName == "" || len([]rune(in.SourceName)) > 100 || in.BaseURL == "" || len(in.BaseURL) > 500 {
		return errors.New("invalid forum source")
	}
	u, err := url.Parse(in.BaseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Fragment != "" {
		return errors.New("invalid forum source url")
	}
	charset := strings.TrimSpace(in.RequestCharset)
	if charset == "" || len(charset) > 40 {
		return errors.New("invalid request charset")
	}
	if err := ValidateCookieSecretRef(in.CookieSecretRef); err != nil {
		return err
	}
	if len(in.UserAgent) > 500 || len([]rune(in.Remark)) > 500 {
		return errors.New("forum source field too long")
	}
	interval, err := strconv.Atoi(in.RequestIntervalMs)
	if err != nil || interval < 100 || interval > 86400000 {
		return errors.New("invalid request interval")
	}
	order, err := strconv.Atoi(in.SortOrder)
	if err != nil || order < 0 {
		return errors.New("invalid sort order")
	}
	return nil
}

func normalize(in Input) Input {
	in.SourceName = strings.TrimSpace(in.SourceName)
	in.BaseURL = strings.TrimSpace(in.BaseURL)
	in.RequestCharset = strings.TrimSpace(in.RequestCharset)
	in.CookieSecretRef = strings.TrimSpace(in.CookieSecretRef)
	in.UserAgent = strings.TrimSpace(in.UserAgent)
	in.RequestIntervalMs = strings.TrimSpace(in.RequestIntervalMs)
	in.SortOrder = strings.TrimSpace(in.SortOrder)
	in.Remark = strings.TrimSpace(in.Remark)
	return in
}
