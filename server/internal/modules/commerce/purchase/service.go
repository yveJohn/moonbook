package purchase

import "context"

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }
func (s *Service) BuyMembership(ctx context.Context, id int64, product, request string) (Order, error) {
	return s.Repo.BuyMembership(ctx, id, product, request)
}
func (s *Service) BuyChapter(ctx context.Context, id int64, chapter, expected, request string) (ChapterResult, error) {
	return s.Repo.BuyChapter(ctx, id, chapter, expected, request)
}
func (s *Service) BuyBook(ctx context.Context, id int64, book, expected string) (Order, error) {
	return s.Repo.BuyBook(ctx, id, book, expected)
}
