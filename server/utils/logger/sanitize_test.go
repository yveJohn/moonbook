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

func TestSanitizeBodyMasksPaymentCredentials(t *testing.T) {
	body := `{"merchantPid":"merchant-plaintext","pid":"pid-plaintext","secret":"secret-plaintext","pidConfigured":true}`
	got := SanitizeBody("application/json", body)
	for _, forbidden := range []string{"merchant-plaintext", "pid-plaintext", "secret-plaintext"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("payment credential was not masked: %s", got)
		}
	}
	if !strings.Contains(got, `"pidConfigured":true`) {
		t.Fatalf("non-secret status was unexpectedly masked: %s", got)
	}
}

func TestSanitizeBodyMasksServerChanSystemParameterValue(t *testing.T) {
	body := `{"name":"Server酱 SendKey","key":"serverchan.send_key","value":"SCT_plaintext_secret","desc":"反馈通知"}`
	got := SanitizeBody("application/json", body)
	if strings.Contains(got, "SCT_plaintext_secret") || !strings.Contains(got, `"value":"***"`) {
		t.Fatalf("ServerChan SendKey was not masked: %s", got)
	}
	ordinary := `{"key":"reader.invite.shareText","value":"公开邀请文案"}`
	if got = SanitizeBody("application/json", ordinary); !strings.Contains(got, "公开邀请文案") {
		t.Fatalf("ordinary parameter was unexpectedly masked: %s", got)
	}
}
