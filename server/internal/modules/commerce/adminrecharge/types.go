package adminrecharge

import "context"

type Product struct {
	ID, DiamondAmount                  int64
	ProductName, PriceUSDT, SaleStatus string
	SortOrder                          int
}
type Input struct {
	ProductName, DiamondAmount, PriceUSDT, SaleStatus string
	SortOrder                                         int
}
type Repository interface {
	List(context.Context, string, int, int) ([]Product, int64, error)
	Create(context.Context, Input) (Product, error)
	Update(context.Context, int64, Input) (Product, error)
	Delete(context.Context, int64) error
}
