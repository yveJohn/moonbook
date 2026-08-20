package crawlsource

import (
	"context"
	"testing"
)

type rejectingRepository struct{ Repository }

func (rejectingRepository) Create(context.Context, Input) (Source, error) { return Source{}, nil }

func TestServiceRejectsCookiePlaintextAndInvalidReference(t *testing.T) {
	service := NewService(rejectingRepository{}, nil, nil)
	plaintext := "session=plaintext"
	base := Input{SourceName: "Forum", BaseURL: "https://forum.example.test", RequestCharset: "UTF-8", RequestIntervalMs: "1000", SortOrder: "0"}

	withPlaintext := base
	withPlaintext.CookieText = &plaintext
	if _, err := service.Create(context.Background(), withPlaintext); err == nil || err.Error() != "forum cookie plaintext is not accepted" {
		t.Fatalf("plaintext error=%v", err)
	}

	withInvalidRef := base
	withInvalidRef.CookieSecretRef = "OTHER_COOKIE"
	if _, err := service.Create(context.Background(), withInvalidRef); err != ErrInvalidCookieSecretRef {
		t.Fatalf("invalid reference error=%v", err)
	}
}

type checkRepository struct {
	rejectingRepository
	source Source
}

func (r checkRepository) Get(context.Context, int64) (Source, error) { return r.source, nil }

type checkSecretResolver struct {
	value string
	err   error
}

func (r checkSecretResolver) ResolveCookie(string) (string, error) { return r.value, r.err }

type recordingChecker struct {
	called bool
	target CheckTarget
}

func (c *recordingChecker) Check(_ context.Context, target CheckTarget) CheckResult {
	c.called, c.target = true, target
	return CheckResult{OK: true, Code: CheckCodeOK}
}

func TestServiceCheckDoesNotCallCheckerWhenSecretIsMissing(t *testing.T) {
	checker := &recordingChecker{}
	service := NewService(
		checkRepository{source: Source{BaseURL: "https://forum.example.test", CookieSecretRef: "MOONBOOK_FORUM_COOKIE_TEST"}},
		checkSecretResolver{err: ErrCookieSecretNotConfigured},
		checker,
	)
	result, err := service.Check(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != CheckCodeSecretNotConfigured || result.OK || checker.called {
		t.Fatalf("result=%+v checker.called=%v", result, checker.called)
	}
}

func TestServiceCheckPassesResolvedSecretOnlyToChecker(t *testing.T) {
	checker := &recordingChecker{}
	service := NewService(
		checkRepository{source: Source{BaseURL: "https://forum.example.test", UserAgent: "MoonbookCheck/1", CookieSecretRef: "MOONBOOK_FORUM_COOKIE_TEST"}},
		checkSecretResolver{value: "session=test-value"},
		checker,
	)
	result, err := service.Check(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || !checker.called {
		t.Fatalf("result=%+v checker.called=%v", result, checker.called)
	}
	if checker.target.BaseURL != "https://forum.example.test" || checker.target.UserAgent != "MoonbookCheck/1" || checker.target.Cookie != "session=test-value" {
		t.Fatal("checker did not receive the expected source credentials")
	}
}
