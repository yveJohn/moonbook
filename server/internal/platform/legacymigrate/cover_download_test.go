package legacymigrate

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCoverDownloaderSecurityAndLimits(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\nfixture")
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/cover":
			writer.Header().Set("Content-Type", "application/octet-stream")
			_, _ = writer.Write(png)
		case "/redirect":
			http.Redirect(writer, request, "/cover", http.StatusFound)
		case "/large":
			writer.Header().Set("Content-Length", "10485761")
			writer.WriteHeader(http.StatusOK)
		default:
			http.NotFound(writer, request)
		}
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()

	blocked, err := NewCoverDownloader(time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := blocked.Download(context.Background(), "http://127.0.0.1/cover"); err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("loopback fixture should be blocked, got %v", err)
	}

	allowed, err := NewCoverDownloader(time.Second, []string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/cover", "/redirect"} {
		data, err := allowed.Download(context.Background(), server.URL+path)
		if err != nil || string(data) != string(png) {
			t.Fatalf("download %s data=%q err=%v", path, data, err)
		}
	}
	if _, err := allowed.Download(context.Background(), server.URL+"/large"); err == nil || !strings.Contains(err.Error(), "10 MiB") {
		t.Fatalf("oversized cover should fail, got %v", err)
	}
	if _, err := allowed.Download(context.Background(), "file:///tmp/cover.png"); err == nil {
		t.Fatal("file URL should be rejected")
	}
}

func TestCoverDownloadEnvironmentParsing(t *testing.T) {
	hosts := ParseAllowedCoverHosts(" covers.example.com,127.0.0.1 ,, ")
	if len(hosts) != 2 || hosts[0] != "covers.example.com" || hosts[1] != "127.0.0.1" {
		t.Fatalf("hosts=%v", hosts)
	}
	if timeout, err := ParseCoverDownloadTimeout(""); err != nil || timeout != 20*time.Second {
		t.Fatalf("default timeout=%v err=%v", timeout, err)
	}
	for _, value := range []string{"0", "121", "invalid"} {
		if _, err := ParseCoverDownloadTimeout(value); err == nil {
			t.Fatalf("timeout %q should fail", value)
		}
	}
}
