package crawlsource

import (
	"context"
	"testing"
)

type rejectingRepository struct{ Repository }

func (rejectingRepository) Create(context.Context, Input) (Source, error) { return Source{}, nil }

func TestServiceRejectsCookiePlaintextAndInvalidReference(t *testing.T) {
	service := NewService(rejectingRepository{})
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
