package adminrecharge

import (
	"context"
	"database/sql"
	"errors"
	"math/big"
	"strconv"
	"strings"
)

type SQLRepository struct{ DB *sql.DB }

var ErrInvalid = errors.New("invalid recharge product")

func normalize(in Input) (Input, error) {
	in.ProductName = strings.TrimSpace(in.ProductName)
	in.DiamondAmount = strings.TrimSpace(in.DiamondAmount)
	in.PriceUSDT = strings.TrimSpace(in.PriceUSDT)
	if in.ProductName == "" || len([]rune(in.ProductName)) > 100 {
		return Input{}, ErrInvalid
	}
	d, e := strconv.ParseInt(in.DiamondAmount, 10, 64)
	if e != nil || d <= 0 {
		return Input{}, ErrInvalid
	}
	r, ok := new(big.Rat).SetString(in.PriceUSDT)
	if !ok || r.Sign() <= 0 {
		return Input{}, ErrInvalid
	}
	if in.SaleStatus != "on_sale" && in.SaleStatus != "off_sale" {
		return Input{}, ErrInvalid
	}
	if in.SortOrder < 0 {
		return Input{}, ErrInvalid
	}
	return in, nil
}
func (r SQLRepository) List(ctx context.Context, k string, p, n int) ([]Product, int64, error) {
	var total int64
	if e := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM reader_recharge_products WHERE ($1='' OR product_name ILIKE '%'||$1||'%')`, k).Scan(&total); e != nil {
		return nil, 0, e
	}
	rows, e := r.DB.QueryContext(ctx, `SELECT id,product_name,diamond_amount,price_usdt::text,sale_status,sort_order FROM reader_recharge_products WHERE ($1='' OR product_name ILIKE '%'||$1||'%') ORDER BY sort_order,id LIMIT $2 OFFSET $3`, k, n, (p-1)*n)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	out := []Product{}
	for rows.Next() {
		var v Product
		if e = rows.Scan(&v.ID, &v.ProductName, &v.DiamondAmount, &v.PriceUSDT, &v.SaleStatus, &v.SortOrder); e != nil {
			return nil, 0, e
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}
func (r SQLRepository) Create(ctx context.Context, in Input) (Product, error) {
	in, e := normalize(in)
	if e != nil {
		return Product{}, e
	}
	var v Product
	e = r.DB.QueryRowContext(ctx, `INSERT INTO reader_recharge_products(product_name,diamond_amount,price_usdt,sale_status,sort_order) VALUES($1,$2,$3,$4,$5) RETURNING id,product_name,diamond_amount,price_usdt::text,sale_status,sort_order`, in.ProductName, in.DiamondAmount, in.PriceUSDT, in.SaleStatus, in.SortOrder).Scan(&v.ID, &v.ProductName, &v.DiamondAmount, &v.PriceUSDT, &v.SaleStatus, &v.SortOrder)
	return v, e
}
func (r SQLRepository) Update(ctx context.Context, id int64, in Input) (Product, error) {
	if id <= 0 {
		return Product{}, ErrInvalid
	}
	in, e := normalize(in)
	if e != nil {
		return Product{}, e
	}
	var v Product
	e = r.DB.QueryRowContext(ctx, `UPDATE reader_recharge_products SET product_name=$1,diamond_amount=$2,price_usdt=$3,sale_status=$4,sort_order=$5,updated_at=now() WHERE id=$6 RETURNING id,product_name,diamond_amount,price_usdt::text,sale_status,sort_order`, in.ProductName, in.DiamondAmount, in.PriceUSDT, in.SaleStatus, in.SortOrder, id).Scan(&v.ID, &v.ProductName, &v.DiamondAmount, &v.PriceUSDT, &v.SaleStatus, &v.SortOrder)
	return v, e
}
func (r SQLRepository) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalid
	}
	res, e := r.DB.ExecContext(ctx, `DELETE FROM reader_recharge_products WHERE id=$1`, id)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
