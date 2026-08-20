package crawlsource

import (
	"errors"
	"strings"
	"testing"
)

func TestEnvSecretResolver(t *testing.T) {
	lookup := func(key string) (string, bool) {
		if key == "MOONBOOK_FORUM_COOKIE_EXAMPLE" {
			return "session=fixture", true
		}
		return "", false
	}
	resolver := EnvSecretResolver{Lookup: lookup}

	for _, test := range []struct {
		name, ref, want string
		err             error
	}{
		{name: "empty reference", ref: ""},
		{name: "resolved", ref: "MOONBOOK_FORUM_COOKIE_EXAMPLE", want: "session=fixture"},
		{name: "invalid reference", ref: "OTHER_COOKIE", err: ErrInvalidCookieSecretRef},
		{name: "missing value", ref: "MOONBOOK_FORUM_COOKIE_MISSING", err: ErrCookieSecretNotConfigured},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolver.ResolveCookie(test.ref)
			if got != test.want || !errors.Is(err, test.err) {
				t.Fatalf("ResolveCookie() configured=%t err=%v, want configured=%t err=%v", got != "", err, test.want != "", test.err)
			}
			if err != nil && (strings.Contains(err.Error(), test.ref) || strings.Contains(err.Error(), "session=fixture")) {
				t.Fatalf("error exposes secret metadata: %v", err)
			}
		})
	}
}

func TestEnvSecretResolverRejectsUnsafeValues(t *testing.T) {
	for _, value := range []string{"header\r\ninjection", strings.Repeat("x", MaxCookieSecretBytes+1)} {
		resolver := EnvSecretResolver{Lookup: func(string) (string, bool) { return value, true }}
		if _, err := resolver.ResolveCookie("MOONBOOK_FORUM_COOKIE_EXAMPLE"); !errors.Is(err, ErrInvalidCookieSecret) {
			t.Fatalf("ResolveCookie() error=%v", err)
		}
	}
}
