package crawlsource

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"
)

type fakeCheckResolver struct {
	addresses []netip.Addr
	err       error
}

func (r fakeCheckResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return r.addresses, r.err
}

func newLoopbackChecker(t *testing.T, timeout time.Duration, maxBytes int64, hosts ...string) *HTTPSourceChecker {
	t.Helper()
	checker, err := NewHTTPSourceChecker(timeout, maxBytes, hosts)
	if err != nil {
		t.Fatal(err)
	}
	return checker
}

func TestHTTPSourceCheckerBlocksUnsafeTargets(t *testing.T) {
	checker := NewDefaultHTTPSourceChecker()
	for _, target := range []string{"http://127.0.0.1", "http://example.com:8080", "ftp://example.com/file", "https://user@example.com"} {
		result := checker.Check(context.Background(), CheckTarget{BaseURL: target})
		if result.Code != CheckCodeTargetBlocked || result.OK {
			t.Fatalf("target %q result=%+v", target, result)
		}
	}
}

func TestHTTPSourceCheckerSendsConfiguredHeadersAndReturnsOnlyStableResult(t *testing.T) {
	const userAgent = "MoonbookConnectionCheck/1"
	const cookie = "session=private-test-value"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.UserAgent() != userAgent || request.Header.Get("Cookie") != cookie {
			t.Error("request did not contain configured headers")
		}
		_, _ = writer.Write([]byte("private response body"))
	}))
	defer server.Close()

	checker := newLoopbackChecker(t, time.Second, 1024, "127.0.0.1")
	result := checker.Check(context.Background(), CheckTarget{BaseURL: server.URL, UserAgent: userAgent, Cookie: cookie})
	if !result.OK || result.Code != CheckCodeOK || result.HTTPStatus != http.StatusOK || result.ElapsedMs < 0 {
		t.Fatalf("result=%+v", result)
	}
	formatted := fmt.Sprintf("%+v", result)
	if strings.Contains(formatted, cookie) || strings.Contains(formatted, "private response body") || strings.Contains(formatted, server.URL) {
		t.Fatal("check result exposed request or response secrets")
	}
}

func TestHTTPSourceCheckerClassifiesHTTPStatuses(t *testing.T) {
	for _, test := range []struct {
		status int
		code   string
	}{
		{http.StatusUnauthorized, CheckCodeAuthFailed},
		{http.StatusForbidden, CheckCodeAuthFailed},
		{http.StatusInternalServerError, CheckCodeHTTPError},
	} {
		t.Run(test.code+fmt.Sprint(test.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(test.status) }))
			defer server.Close()
			result := newLoopbackChecker(t, time.Second, 1024, "127.0.0.1").Check(context.Background(), CheckTarget{BaseURL: server.URL})
			if result.Code != test.code || result.HTTPStatus != test.status || result.OK {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}

func TestHTTPSourceCheckerEnforcesRedirectPolicy(t *testing.T) {
	t.Run("cross host", func(t *testing.T) {
		destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		defer destination.Close()
		source := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			redirectURL, _ := url.Parse(destination.URL)
			redirectURL.Host = "localhost:" + redirectURL.Port()
			http.Redirect(writer, request, redirectURL.String(), http.StatusFound)
		}))
		defer source.Close()
		result := newLoopbackChecker(t, time.Second, 1024, "127.0.0.1", "localhost").Check(context.Background(), CheckTarget{BaseURL: source.URL})
		if result.Code != CheckCodeRedirectBlocked || result.OK {
			t.Fatalf("result=%+v", result)
		}
	})

	t.Run("same host", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path == "/start" {
				http.Redirect(writer, request, "/done", http.StatusFound)
				return
			}
			_, _ = writer.Write([]byte("ok"))
		}))
		defer server.Close()
		result := newLoopbackChecker(t, time.Second, 1024, "127.0.0.1").Check(context.Background(), CheckTarget{BaseURL: server.URL + "/start"})
		if !result.OK || result.Code != CheckCodeOK {
			t.Fatalf("result=%+v", result)
		}
	})
}

func TestHTTPSourceCheckerEnforcesTimeoutAndResponseLimit(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			time.Sleep(100 * time.Millisecond)
			_, _ = writer.Write([]byte("late"))
		}))
		defer server.Close()
		result := newLoopbackChecker(t, 20*time.Millisecond, 1024, "127.0.0.1").Check(context.Background(), CheckTarget{BaseURL: server.URL})
		if result.Code != CheckCodeTimeout || result.OK {
			t.Fatalf("result=%+v", result)
		}
	})

	for _, test := range []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"content length", func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Length", "5")
			writer.WriteHeader(http.StatusOK)
		}},
		{"streamed", func(writer http.ResponseWriter, _ *http.Request) {
			writer.(http.Flusher).Flush()
			_, _ = writer.Write([]byte("12345"))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(test.handler)
			defer server.Close()
			result := newLoopbackChecker(t, time.Second, 4, "127.0.0.1").Check(context.Background(), CheckTarget{BaseURL: server.URL})
			if result.Code != CheckCodeResponseTooLarge || result.OK {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}

func TestHTTPSourceCheckerClassifiesDNSAndConnectionFailures(t *testing.T) {
	checker := newLoopbackChecker(t, time.Second, 1024)
	checker.resolver = fakeCheckResolver{err: errors.New("test lookup failure")}
	if result := checker.Check(context.Background(), CheckTarget{BaseURL: "https://missing.example.test"}); result.Code != CheckCodeDNSFailed {
		t.Fatalf("DNS result=%+v", result)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	result := newLoopbackChecker(t, time.Second, 1024, "127.0.0.1").Check(context.Background(), CheckTarget{BaseURL: "http://" + address})
	if result.Code != CheckCodeConnectFailed || result.OK {
		t.Fatalf("connection result=%+v", result)
	}
}

func TestRenderCheckContainsNoTransportDetails(t *testing.T) {
	rendered := renderCheck(CheckResult{OK: false, Code: CheckCodeHTTPError, HTTPStatus: 500, ElapsedMs: 12})
	if len(rendered) != 4 || rendered["code"] != CheckCodeHTTPError {
		t.Fatalf("rendered=%+v", rendered)
	}
	for _, forbidden := range []string{"url", "cookie", "body", "error", "ip"} {
		if _, exists := rendered[forbidden]; exists {
			t.Fatalf("rendered response contains %q", forbidden)
		}
	}
}
