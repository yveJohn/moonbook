package logger

import (
	"net/http"
	"strings"
	"testing"
)

func TestSanitizeHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer secret")
	h.Set("X-Token", "tok")
	h.Set("Accept", "application/json")
	got := SanitizeHeaders(h)
	if got["Authorization"] != maskValue || got["X-Token"] != maskValue {
		t.Fatalf("sensitive headers not masked: %+v", got)
	}
	if got["Accept"] != "application/json" {
		t.Fatalf("normal header changed: %+v", got)
	}
}

func TestTruncate(t *testing.T) {
	if Truncate("short", 100) != "short" {
		t.Fatal("short should be unchanged")
	}
	long := strings.Repeat("a", 2000)
	if Truncate(long, 1024) != truncatedMark {
		t.Fatal("long should be replaced with mark")
	}
}

func TestSanitizeBodyMasksCookieFields(t *testing.T) {
	body := `{"cookieText":"session=plaintext","nested":{"cookie":"other-secret"},"cookieSecretRef":"MOONBOOK_FORUM_COOKIE_EXAMPLE"}`
	got := SanitizeBody("application/json", body)
	if strings.Contains(got, "session=plaintext") || strings.Contains(got, "other-secret") {
		t.Fatalf("cookie value was not masked: %s", got)
	}
	if !strings.Contains(got, "MOONBOOK_FORUM_COOKIE_EXAMPLE") {
		t.Fatalf("non-secret reference was unexpectedly masked: %s", got)
	}
}
