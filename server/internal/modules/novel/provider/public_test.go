package provider

import (
	"context"
	"errors"
	"testing"

	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
)

func TestPublicProviderRejectsUnavailableDependencies(t *testing.T) {
	provider := NewPublic(nil, nil)
	if _, err := provider.FeaturedBooks(context.Background()); !errors.Is(err, novelcontract.ErrUnavailable) {
		t.Fatalf("featured error=%v", err)
	}
	if _, err := provider.Book(context.Background(), 1); !errors.Is(err, novelcontract.ErrBookNotFound) {
		t.Fatalf("book error=%v", err)
	}
	if _, err := provider.ChapterContent(context.Background(), 1); !errors.Is(err, novelcontract.ErrObjectUnavailable) {
		t.Fatalf("content error=%v", err)
	}
}
