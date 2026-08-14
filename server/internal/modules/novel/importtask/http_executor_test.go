package importtask

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPExecutorWithLocalSubstitute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing user agent")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<h1>帖子</h1><p>第一章\n正文</p><p>Chapter 2</p>"))
	}))
	defer server.Close()
	e, err := NewHTTPExecutor(time.Second, 1024)
	if err != nil {
		t.Fatal(err)
	}
	result, err := e.Execute(context.Background(), Task{ThreadURL: server.URL + "/thread"})
	if err != nil || result.TotalChapterCount != 2 || result.QualityStatus != "passed" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
func TestHTTPExecutorErrorsAndLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
