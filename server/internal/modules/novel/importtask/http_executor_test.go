package importtask

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestHTTPExecutorWithLocalSubstitute(t *testing.T) {
	requests := []string{}
	server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		if r.Header.Get("User-Agent") != "Forum-Fixture/1.0" || r.Header.Get("Cookie") != "session=fixture" {
			t.Errorf("unexpected source headers: user-agent=%q cookie=%q", r.Header.Get("User-Agent"), r.Header.Get("Cookie"))
		}
		switch r.URL.Path {
		case "/thread/1":
			_, _ = w.Write([]byte(`<h1>第一章 开始</h1><p>第一页正文</p><a rel="next" href="/thread/2">下一页</a>`))
		case "/thread/2":
			_, _ = w.Write([]byte(`<h1>第二章 继续</h1><p>第二页正文</p>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	e, err := NewHTTPExecutor(time.Second, 4096)
	if err != nil {
		t.Fatal(err)
	}
	task := Task{ThreadURL: server.URL + "/thread/1", sourceUserAgent: "Forum-Fixture/1.0", sourceCookie: "session=fixture", requestInterval: time.Millisecond}
	result, err := e.Execute(context.Background(), task)
	if err != nil || result.TotalChapterCount != 2 || result.QualityStatus != "passed" || len(result.Fetches) != 2 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if !reflect.DeepEqual(requests, []string{"/thread/1", "/thread/2"}) {
		t.Fatalf("requests=%v", requests)
	}
	if result.Fetches[0].Status != "succeeded" || result.Fetches[1].Status != "succeeded" || result.Fetches[1].Elapsed <= 0 {
		t.Fatalf("fetches=%+v", result.Fetches)
	}
}
func TestHTTPExecutorErrorsAndLimit(t *testing.T) {
	server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/retry") {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("0123456789"))
	}))
	defer server.Close()
	e, _ := NewHTTPExecutor(time.Second, 5)
	if _, err := e.Execute(context.Background(), Task{ThreadURL: server.URL + "/retry"}); err == nil {
		t.Fatal("expected retryable error")
	}
	if _, err := e.Execute(context.Background(), Task{ThreadURL: server.URL + "/large"}); err == nil {
		t.Fatal("expected response size error")
	}
}

func TestHTTPExecutorPaginationGuards(t *testing.T) {
	server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/external":
			_, _ = w.Write([]byte(`<h1>第一章</h1><a rel="next" href="https://outside.example/page/2">下一页</a>`))
		case "/cycle/1":
			_, _ = w.Write([]byte(`<h1>第一章</h1><a class="nxt" href="/cycle/2">下一页</a>`))
		case "/cycle/2":
			_, _ = w.Write([]byte(`<h1>第二章</h1><a class="nxt" href="/cycle/1">下一页</a>`))
		case "/limit/1":
			_, _ = w.Write([]byte(`<h1>第一章</h1><a rel="next" href="/limit/2">下一页</a>`))
		case "/limit/2":
			_, _ = w.Write([]byte(`<h1>第二章</h1>`))
		}
	}))
	defer server.Close()
	executor, _ := NewHTTPExecutor(time.Second, 4096)
	for _, path := range []string{"/external", "/cycle/1"} {
		if _, err := executor.Execute(context.Background(), Task{ThreadURL: server.URL + path}); err == nil {
			t.Fatalf("expected pagination guard for %s", path)
		}
	}
	executor.MaxPages = 1
	result, err := executor.Execute(context.Background(), Task{ThreadURL: server.URL + "/limit/1"})
	if err == nil || len(result.Fetches) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestHTTPExecutorKeepsSuccessfulPagesOnRetryableFailure(t *testing.T) {
	server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/page/1" {
			_, _ = w.Write([]byte(`<h1>第一章</h1><a rel="next" href="/page/2">下一页</a>`))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	executor, _ := NewHTTPExecutor(time.Second, 4096)
	result, err := executor.Execute(context.Background(), Task{ThreadURL: server.URL + "/page/1"})
	var retryable RetryableError
	if !errors.As(err, &retryable) || len(result.Fetches) != 2 || result.Fetches[0].Status != "succeeded" || result.Fetches[1].Status != "failed" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func newHTTPTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.Start()
	return server
}
