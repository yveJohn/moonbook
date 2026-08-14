package candidate

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var threadPatterns = []*regexp.Regexp{regexp.MustCompile(`(?i)(?:thread-|tid=)([0-9]+)`), regexp.MustCompile(`(?i)(?:threadid=|topicid=)([0-9]+)`)}
var hrefPattern = regexp.MustCompile(`(?is)<a\b[^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`)
var tagPattern = regexp.MustCompile(`(?is)<[^>]+>`)

func DiscoverBoard(ctx context.Context, target BoardTarget, client *http.Client) ([]Discovered, error) {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	base, err := url.Parse(target.BoardURL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Hostname() == "" {
		return nil, fmt.Errorf("invalid board url")
	}
	pages := []string{target.BoardURL}
	if target.BoardURLTemplate != "" {
		pages = pages[:0]
		for i := 1; i <= 10; i++ {
			pages = append(pages, strings.ReplaceAll(target.BoardURLTemplate, "{page}", fmt.Sprint(i)))
		}
	}
	seen := map[string]Discovered{}
	for _, pageURL := range pages {
		u, e := url.Parse(pageURL)
		if e != nil || u.Hostname() != base.Hostname() {
			return nil, fmt.Errorf("page url host differs from board")
		}
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
		if e != nil {
			return nil, e
		}
		if target.UserAgent != "" {
			req.Header.Set("User-Agent", target.UserAgent)
		}
		resp, e := client.Do(req)
		if e != nil {
			return nil, e
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			return nil, fmt.Errorf("board fetch returned %s", resp.Status)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			continue
		}
		for _, item := range parseDiscovered(body, u, base) {
			seen[item.ForumThreadID] = item
		}
	}
	out := make([]Discovered, 0, len(seen))
	for _, item := range seen {
		out = append(out, item)
	}
	return out, nil
}

func parseDiscovered(body []byte, page, base *url.URL) []Discovered {
	seen := map[string]Discovered{}
	for _, match := range hrefPattern.FindAllSubmatch(body, -1) {
		href := strings.TrimSpace(string(match[1]))
		threadID := forumThreadID(href)
		if threadID == "" {
			continue
		}
		resolved, err := page.Parse(href)
		if err != nil || (resolved.IsAbs() && resolved.Hostname() != base.Hostname()) {
			continue
		}
		full := page.ResolveReference(resolved).String()
		title := strings.TrimSpace(tagPattern.ReplaceAllString(string(match[2]), ""))
		if title == "" {
			title = threadID
		}
		seen[threadID] = Discovered{ForumThreadID: threadID, ThreadTitle: title, ThreadURL: full}
	}
	out := make([]Discovered, 0, len(seen))
	for _, item := range seen {
		out = append(out, item)
	}
	return out
}

func forumThreadID(raw string) string {
	for _, p := range threadPatterns {
		if m := p.FindStringSubmatch(raw); len(m) > 1 {
			return m[1]
		}
	}
	return ""
}
