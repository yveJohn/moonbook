package recharge

import "context"

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service                       { return &Service{Repo: repo} }
func (s *Service) Catalog(ctx context.Context) (Catalog, error) { return s.Repo.Catalog(ctx) }
func (s *Service) Quote(ctx context.Context, amount int64) (Quote, error) {
	return s.Repo.Quote(ctx, amount)
}
func (s *Service) CreateOrder(ctx context.Context, req CreateRequest) (Order, error) {
	return s.Repo.CreateOrder(ctx, req)
}
func (s *Service) GetOrder(ctx context.Context, readerID int64, orderID string) (Order, error) {
	return s.Repo.GetOrder(ctx, readerID, orderID)
}
