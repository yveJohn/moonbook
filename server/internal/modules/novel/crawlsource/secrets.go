package crawlsource

import (
	"errors"
	"regexp"
	"strings"
)

const MaxCookieSecretBytes = 8192

var (
	CookieSecretRefPattern       = regexp.MustCompile(`^MOONBOOK_FORUM_COOKIE_[A-Z0-9_]+$`)
	ErrInvalidCookieSecretRef    = errors.New("invalid forum cookie secret reference")
	ErrCookieSecretNotConfigured = errors.New("forum cookie secret is not configured")
	ErrInvalidCookieSecret       = errors.New("forum cookie secret is invalid")
)

type SecretResolver interface {
	ResolveCookie(string) (string, error)
}

type EnvSecretResolver struct {
	Lookup func(string) (string, bool)
}

func ValidateCookieSecretRef(ref string) error {
	if ref == "" {
		return nil
	}
	if len(ref) > 128 || !CookieSecretRefPattern.MatchString(ref) {
		return ErrInvalidCookieSecretRef
	}
	return nil
}

func (r EnvSecretResolver) ResolveCookie(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if err := ValidateCookieSecretRef(ref); err != nil {
		return "", err
	}
	if ref == "" {
		return "", nil
	}
	if r.Lookup == nil {
		return "", ErrCookieSecretNotConfigured
	}
	value, ok := r.Lookup(ref)
	if !ok || strings.TrimSpace(value) == "" {
		return "", ErrCookieSecretNotConfigured
	}
	if len(value) > MaxCookieSecretBytes || strings.ContainsAny(value, "\r\n") {
		return "", ErrInvalidCookieSecret
	}
	return value, nil
}
