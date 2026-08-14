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
type Setting struct {
	ID                                                  int64
	CustomEnabled                                       bool
	DiamondsPerUSDT, MinDiamondAmount, MaxDiamondAmount string
	AmountScale                                         int
	RoundingMode                                        string
}
type SettingInput struct {
	CustomEnabled                                       bool
	DiamondsPerUSDT, MinDiamondAmount, MaxDiamondAmount string
}
type Repository interface {
	List(context.Context, string, int, int) ([]Product, int64, error)
	Create(context.Context, Input) (Product, error)
	Update(context.Context, int64, Input) (Product, error)
	Delete(context.Context, int64) error
	GetSetting(context.Context) (Setting, error)
	UpdateSetting(context.Context, SettingInput) (Setting, error)
}
