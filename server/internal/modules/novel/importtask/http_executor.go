package importtask

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultMaxThreadPages = 50

var errCrossOriginRedirect = errors.New("forum redirect leaves thread origin")

type HTTPExecutor struct {
	Client           *http.Client
	MaxResponseBytes int64
	MaxPages         int
	UserAgent        string
}

func NewHTTPExecutor(timeout time.Duration, maxResponseBytes int64) (*HTTPExecutor, error) {
	if timeout <= 0 {
		return nil, errors.New("timeout must be positive")
	}
	if maxResponseBytes <= 0 {
		return nil, errors.New("max response bytes must be positive")
	}
	return &HTTPExecutor{Client: &http.Client{Timeout: timeout}, MaxResponseBytes: maxResponseBytes, MaxPages: defaultMaxThreadPages}, nil
}

func (e *HTTPExecutor) Execute(ctx context.Context, task Task) (Result, error) {
	first, err := parseThreadURL(task.ThreadURL)
	if err != nil {
		return Result{}, err
	}
	if e.Client == nil {
		return Result{}, errors.New("http client is required")
	}
	maxPages := e.MaxPages
	if maxPages <= 0 {
		maxPages = defaultMaxThreadPages
	}
	client := *e.Client
	previousRedirectCheck := client.CheckRedirect
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if !sameOrigin(request.URL, first) {
			return errCrossOriginRedirect
		}
		if previousRedirectCheck != nil {
			return previousRedirectCheck(request, via)
		}
		if len(via) >= 10 {
			return errors.New("too many forum redirects")
		}
		return nil
	}

	result := Result{}
	remaining := e.MaxResponseBytes
	visited := map[string]bool{}
	current := first
	var combined bytes.Buffer
	for pageNo := 1; ; pageNo++ {
		key := current.String()
		if visited[key] {
			return result, errors.New("forum pagination cycle detected")
		}
		if pageNo > maxPages {
			return result, errors.New("forum pagination exceeds page limit")
		}
		visited[key] = true
		if pageNo > 1 {
			if err := waitForPage(ctx, task.requestInterval); err != nil {
				return result, err
			}
		}
		body, next, fetch, fetchErr := e.fetchPage(ctx, &client, task, current, first, pageNo, remaining)
		result.Fetches = append(result.Fetches, fetch)
		if fetchErr != nil {
			return result, fetchErr
		}
		remaining -= int64(len(body))
		combined.Write(body)
		combined.WriteByte('\n')
		if next == nil {
			break
		}
		current = next
	}

	parsed := deduplicateChapters(ParseForumChapters(combined.Bytes()))
	chapters := len(parsed)
	quality := "passed"
	summary := fmt.Sprintf("抓取 %d 页、响应 %d 字节，识别章节 %d 个", len(result.Fetches), combined.Len(), chapters)
	if chapters == 0 {
		quality = "warning"
		summary = fmt.Sprintf("抓取 %d 页成功但未识别章节标记", len(result.Fetches))
	}
	result.TotalChapterCount = chapters
	result.QualityStatus = quality
	result.QualitySummary = summary
	result.Chapters = parsed
	return result, nil
}

func (e *HTTPExecutor) fetchPage(ctx context.Context, client *http.Client, task Task, current, origin *url.URL, pageNo int, remaining int64) (body []byte, next *url.URL, fetch PageFetch, err error) {
	fetch = PageFetch{URL: current.String(), PageNo: pageNo, Status: "failed"}
	started := time.Now()
	defer func() { fetch.Elapsed = time.Since(started) }()
	if remaining <= 0 {
		fetch.Message = "论坛分页响应超过总大小限制"
		return nil, nil, fetch, errors.New("forum responses exceed total size limit")
	}
	req, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, current.String(), nil)
	if requestErr != nil {
		fetch.Message = requestErr.Error()
		return nil, nil, fetch, requestErr
	}
	ua := strings.TrimSpace(task.sourceUserAgent)
	if ua == "" {
		ua = strings.TrimSpace(e.UserAgent)
	}
	if ua == "" {
		ua = "Moonbook-Crawler/1.0"
	}
	req.Header.Set("User-Agent", ua)
	if cookie := strings.TrimSpace(task.sourceCookie); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, requestErr := client.Do(req)
	if requestErr != nil {
		fetch.Message = requestErr.Error()
		if errors.Is(requestErr, errCrossOriginRedirect) {
			return nil, nil, fetch, errCrossOriginRedirect
		}
		return nil, nil, fetch, RetryableError{Err: fmt.Errorf("fetch forum thread page %d: %w", pageNo, requestErr)}
	}
	defer resp.Body.Close()
	fetch.HTTPStatus = resp.StatusCode
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		fetch.Message = fmt.Sprintf("论坛返回 HTTP %d", resp.StatusCode)
		return nil, nil, fetch, RetryableError{Err: fmt.Errorf("forum returned HTTP %d on page %d", resp.StatusCode, pageNo)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fetch.Message = fmt.Sprintf("论坛返回 HTTP %d", resp.StatusCode)
		return nil, nil, fetch, fmt.Errorf("forum returned HTTP %d on page %d", resp.StatusCode, pageNo)
	}
	body, err = io.ReadAll(io.LimitReader(resp.Body, remaining+1))
	if err != nil {
		fetch.Message = err.Error()
		return nil, nil, fetch, RetryableError{Err: fmt.Errorf("read forum response page %d: %w", pageNo, err)}
	}
	fetch.ResponseBytes = len(body)
	if int64(len(body)) > remaining {
		fetch.Message = "论坛分页响应超过总大小限制"
		return nil, nil, fetch, errors.New("forum responses exceed total size limit")
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		fetch.Message = "论坛分页响应为空"
		return nil, nil, fetch, fmt.Errorf("forum response page %d is empty", pageNo)
	}
	next, err = findNextPage(body, current, origin)
	if err != nil {
		fetch.Message = err.Error()
		return nil, nil, fetch, err
	}
	fetch.Status = "succeeded"
	fetch.Message = fmt.Sprintf("第 %d 页抓取成功", pageNo)
	return body, next, fetch, nil
}

func parseThreadURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Fragment != "" {
		return nil, errors.New("invalid thread url")
	}
	return u, nil
}

func sameOrigin(left, right *url.URL) bool {
	return strings.EqualFold(left.Scheme, right.Scheme) && strings.EqualFold(left.Host, right.Host)
}

func waitForPage(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return nil
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func deduplicateChapters(items []ParsedChapter) []ParsedChapter {
	seen := map[string]bool{}
	result := make([]ParsedChapter, 0, len(items))
	for _, item := range items {
		key := strings.TrimSpace(item.Title) + "\x00" + strings.TrimSpace(item.Content)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, item)
	}
	return result
}
