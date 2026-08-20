package candidate

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestParseDiscoveredDiscuzLinks(t *testing.T) {
	page, _ := url.Parse("https://forum.example.test/forum")
	base, _ := url.Parse("https://forum.example.test")
	items := parseDiscovered([]byte(`<a href="thread-123-1-1.html">第一帖</a><a href="viewthread.php?tid=456">第二帖</a><a href="https://other.example/thread-999">外站</a>`), page, base)
	if len(items) != 2 {
		t.Fatalf("items=%+v", items)
	}
}

func TestDiscoverBoardSendsResolvedCookie(t *testing.T) {
	var gotCookie string
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		gotCookie = request.Header.Get("Cookie")
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`<a href="thread-123-1-1.html">第一帖</a>`)),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}

	items, err := DiscoverBoard(context.Background(), BoardTarget{BoardURL: "https://forum.example.test", sourceCookie: "session=fixture"}, client)
	if err != nil || len(items) != 1 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	if gotCookie != "session=fixture" {
		t.Fatalf("cookie header configured=%t", gotCookie != "")
	}
}
