package candidate

import (
	"context"
	"net/http"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/crawlsource"
)

type secretTestRepository struct {
	Repository
	target BoardTarget
}

func (r secretTestRepository) GetBoardTarget(context.Context, string) (BoardTarget, error) {
	return r.target, nil
}

func (secretTestRepository) UpsertDiscovered(context.Context, BoardTarget, []Discovered) (DiscoverResult, error) {
	return DiscoverResult{}, nil
}

func TestDiscoverDoesNotRequestWhenCookieSecretIsMissing(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, nil
	})}

	service := NewService(secretTestRepository{target: BoardTarget{
		BoardURL: "https://forum.example.test", Enabled: true, CookieSecretRef: "MOONBOOK_FORUM_COOKIE_MISSING",
	}}, crawlsource.EnvSecretResolver{Lookup: func(string) (string, bool) { return "", false }})
	_, err := service.DiscoverWithClient(context.Background(), "1", client)
	if err != crawlsource.ErrCookieSecretNotConfigured {
		t.Fatalf("error=%v", err)
	}
	if requests != 0 {
		t.Fatalf("sent %d requests without a configured cookie secret", requests)
	}
}
