package candidate

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/crawlsource"
)

type Service struct {
	Repo    Repository
	Secrets crawlsource.SecretResolver
}

func NewService(repo Repository, secrets ...crawlsource.SecretResolver) *Service {
	s := &Service{Repo: repo}
	if len(secrets) > 0 {
		s.Secrets = secrets[0]
	}
	return s
}
func (s *Service) List(ctx context.Context, keyword, sourceID, boardID, status string, page, size int) ([]Candidate, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return s.Repo.List(ctx, keyword, sourceID, boardID, status, page, size)
}
func (s *Service) Get(ctx context.Context, id int64) (Candidate, error) {
	if id <= 0 {
		return Candidate{}, errors.New("invalid candidate id")
	}
	return s.Repo.Get(ctx, id)
}
func (s *Service) Skip(ctx context.Context, ids []int64, requestID string) error {
	return s.transition(ctx, ids, "skipped", requestID)
}
func (s *Service) Restore(ctx context.Context, ids []int64, requestID string) error {
	return s.transition(ctx, ids, "pending", requestID)
}
func (s *Service) transition(ctx context.Context, ids []int64, status, requestID string) error {
	if len(ids) == 0 || len(ids) > 100 {
		return errors.New("invalid candidate ids")
	}
	return s.Repo.Transition(ctx, ids, status, requestID)
}
func (s *Service) Delete(ctx context.Context, ids []int64) error {
	if len(ids) == 0 || len(ids) > 100 {
		return errors.New("invalid candidate ids")
	}
	return s.Repo.Delete(ctx, ids)
}

func (s *Service) Discover(ctx context.Context, boardID string) (DiscoverResult, error) {
	return s.DiscoverWithClient(ctx, boardID, nil)
}

func (s *Service) DiscoverWithClient(ctx context.Context, boardID string, client *http.Client) (DiscoverResult, error) {
	boardID = strings.TrimSpace(boardID)
	if boardID == "" {
		return DiscoverResult{}, errors.New("invalid board id")
	}
	target, err := s.Repo.GetBoardTarget(ctx, boardID)
	if err != nil {
		return DiscoverResult{}, err
	}
	if !target.Enabled {
		return DiscoverResult{}, errors.New("board or source is disabled")
	}
	if err := resolveTargetCookie(&target, s.Secrets); err != nil {
		return DiscoverResult{}, err
	}
	items, err := DiscoverBoard(ctx, target, client)
	if err != nil {
		return DiscoverResult{}, err
	}
	return s.Repo.UpsertDiscovered(ctx, target, items)
}

func resolveTargetCookie(target *BoardTarget, resolver crawlsource.SecretResolver) error {
	if target.CookieSecretRef == "" {
		return nil
	}
	if resolver == nil {
		return crawlsource.ErrCookieSecretNotConfigured
	}
	cookie, err := resolver.ResolveCookie(target.CookieSecretRef)
	if err != nil {
		return err
	}
	target.sourceCookie = cookie
	return nil
}
