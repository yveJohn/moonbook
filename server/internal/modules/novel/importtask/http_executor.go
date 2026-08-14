package importtask

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var chapterMarker = regexp.MustCompile(`(?i)(第\s*[0-9一二三四五六七八九十百千]+\s*章|chapter\s+[0-9]+)`)

type HTTPExecutor struct {
	Client           *http.Client
	MaxResponseBytes int64
	UserAgent        string
}

func NewHTTPExecutor(timeout time.Duration, maxResponseBytes int64) (*HTTPExecutor, error) {
	if timeout <= 0 {
		return nil, errors.New("timeout must be positive")
	}
	if maxResponseBytes <= 0 {
		return nil, errors.New("max response bytes must be positive")
	}
	return &HTTPExecutor{Client: &http.Client{Timeout: timeout}, MaxResponseBytes: maxResponseBytes}, nil
}
func (e *HTTPExecutor) Execute(ctx context.Context, task Task) (Result, error) {
	u, err := url.Parse(task.ThreadURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Fragment != "" {
		return Result{}, errors.New("invalid thread url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, task.ThreadURL, nil)
	if err != nil {
		return Result{}, err
	}
	ua := strings.TrimSpace(e.UserAgent)
	if ua == "" {
		ua = "Moonbook-Crawler/1.0"
	}
	req.Header.Set("User-Agent", ua)
	resp, err := e.Client.Do(req)
	if err != nil {
		return Result{}, RetryableError{Err: fmt.Errorf("fetch forum thread: %w", err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return Result{}, RetryableError{Err: fmt.Errorf("forum returned HTTP %d", resp.StatusCode)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("forum returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, e.MaxResponseBytes+1))
	if err != nil {
		return Result{}, RetryableError{Err: fmt.Errorf("read forum response: %w", err)}
	}
	if int64(len(body)) > e.MaxResponseBytes {
		return Result{}, errors.New("forum response exceeds size limit")
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return Result{}, errors.New("forum response is empty")
	}
	parsed := ParseForumChapters(body)
	chapters := len(parsed)
	quality := "passed"
	summary := fmt.Sprintf("响应 %d 字节，识别章节标记 %d 个", len(body), chapters)
	if chapters == 0 {
		quality = "warning"
		summary = "响应成功但未识别章节标记"
	}
	return Result{TotalChapterCount: chapters, ImportedChapterCount: 0, QualityStatus: quality, QualitySummary: summary, Chapters: parsed}, nil
}
