package adminproduct

import "context"

type Product struct {
	ID, ProductType, TargetID, ProductName, PriceCoin, SaleStatus, SourceType, SourceRef string
	AllowBonusCoin                                                                       bool
	DurationDays                                                                         *int
	SortOrder                                                                            int
}

type Input struct {
	ProductType, TargetID, ProductName, PriceCoin, SaleStatus string
	AllowBonusCoin                                            bool
	DurationDays                                              *int
	SortOrder                                                 int
}

type Repository interface {
	List(context.Context, string, string, string, int, int) ([]Product, int64, error)
	Create(context.Context, Input) (Product, error)
	Update(context.Context, int64, Input) (Product, error)
	Delete(context.Context, int64) error
}
