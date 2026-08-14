package adminproduct

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

type SQLRepository struct{ DB *sql.DB }

func normalize(in Input) (Input, error) {
	in.ProductType = strings.TrimSpace(in.ProductType)
	in.TargetID = strings.TrimSpace(in.TargetID)
	in.ProductName = strings.TrimSpace(in.ProductName)
	in.PriceCoin = strings.TrimSpace(in.PriceCoin)
	in.SaleStatus = strings.TrimSpace(in.SaleStatus)
	if in.ProductType != "book" && in.ProductType != "chapter" && in.ProductType != "membership" && in.ProductType != "ad_free" {
		return Input{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "商品类型无效")
	}
	if in.ProductName == "" || len([]rune(in.ProductName)) > 100 || (in.SaleStatus != "on_sale" && in.SaleStatus != "off_sale") || in.SortOrder < 0 {
		return Input{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "商品参数无效")
	}
	price, err := strconv.ParseInt(in.PriceCoin, 10, 64)
	if err != nil || price < 0 || (in.ProductType == "membership" && price == 0) {
		return Input{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "商品价格无效")
	}
	if in.ProductType == "book" || in.ProductType == "chapter" {
		target, err := strconv.ParseInt(in.TargetID, 10, 64)
		if err != nil || target <= 0 {
			return Input{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "目标 ID 无效")
		}
	} else if in.TargetID != "" {
		return Input{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "该商品类型不接受目标 ID")
	}
	if in.DurationDays != nil && *in.DurationDays <= 0 {
		return Input{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "有效期必须为正整数")
	}
	if in.DurationDays != nil && in.ProductType != "membership" && in.ProductType != "ad_free" {
		return Input{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "该商品类型不接受有效期")
	}
	return in, nil
}

const productSelect = `SELECT id::text,product_type,COALESCE(target_id::text,''),product_name,price_coin::text,allow_bonus_coin, duration_days,sale_status,sort_order,source_type,source_ref FROM commerce_products`

func scan(row interface{ Scan(...any) error }, p *Product) error {
	return row.Scan(&p.ID, &p.ProductType, &p.TargetID, &p.ProductName, &p.PriceCoin, &p.AllowBonusCoin, &p.DurationDays, &p.SaleStatus, &p.SortOrder, &p.SourceType, &p.SourceRef)
}

func (r SQLRepository) List(ctx context.Context, keyword, productType, status string, page, size int) ([]Product, int64, error) {
	where := ` WHERE ($1='' OR product_name ILIKE '%'||$1||'%' OR COALESCE(target_id::text,'')=$1) AND ($2='' OR product_type=$2) AND ($3='' OR sale_status=$3)`
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM commerce_products`+where, keyword, productType, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, productSelect+where+` ORDER BY product_type,sort_order,id LIMIT $4 OFFSET $5`, keyword, productType, status, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]Product, 0)
	for rows.Next() {
		var p Product
		if err := scan(rows, &p); err != nil {
			return nil, 0, err
		}
		items = append(items, p)
	}
	return items, total, rows.Err()
}

func (r SQLRepository) Create(ctx context.Context, in Input) (Product, error) {
	in, err := normalize(in)
	if err != nil {
		return Product{}, err
	}
	if err := r.validateTarget(ctx, in); err != nil {
		return Product{}, err
	}
	var p Product
	err = scan(r.DB.QueryRowContext(ctx, `INSERT INTO commerce_products(product_type,target_id,product_name,price_coin,allow_bonus_coin,duration_days,sale_status,sort_order,source_type,source_ref) VALUES($1,NULLIF($2,'')::bigint,$3,$4,$5,$6,$7,$8,'manual','admin') RETURNING id::text,product_type,COALESCE(target_id::text,''),product_name,price_coin::text,allow_bonus_coin,duration_days,sale_status,sort_order,source_type,source_ref`, in.ProductType, in.TargetID, in.ProductName, in.PriceCoin, in.AllowBonusCoin, in.DurationDays, in.SaleStatus, in.SortOrder), &p)
	return p, err
}

func (r SQLRepository) Update(ctx context.Context, id int64, in Input) (Product, error) {
	if id <= 0 {
		return Product{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串")
	}
	in, err := normalize(in)
	if err != nil {
		return Product{}, err
	}
	var oldType, oldTarget string
	if err := r.DB.QueryRowContext(ctx, `SELECT product_type,COALESCE(target_id::text,'') FROM commerce_products WHERE id=$1`, id).Scan(&oldType, &oldTarget); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Product{}, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "商品不存在")
		}
		return Product{}, err
	}
	if oldType != in.ProductType || oldTarget != in.TargetID {
		var used bool
		if err := r.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reader_purchase_orders WHERE product_id=$1)`, id).Scan(&used); err != nil {
			return Product{}, err
		}
		if used {
			return Product{}, apperror.New(apperror.CodeConflict, http.StatusConflict, "已有消费订单引用该商品，不能修改商品类型或目标")
		}
	}
	if err := r.validateTarget(ctx, in); err != nil {
		return Product{}, err
	}
	var p Product
	err = scan(r.DB.QueryRowContext(ctx, `UPDATE commerce_products SET product_type=$1,target_id=NULLIF($2,'')::bigint,product_name=$3,price_coin=$4,allow_bonus_coin=$5,duration_days=$6,sale_status=$7,sort_order=$8,updated_at=now() WHERE id=$9 RETURNING id::text,product_type,COALESCE(target_id::text,''),product_name,price_coin::text,allow_bonus_coin,duration_days,sale_status,sort_order,source_type,source_ref`, in.ProductType, in.TargetID, in.ProductName, in.PriceCoin, in.AllowBonusCoin, in.DurationDays, in.SaleStatus, in.SortOrder, id), &p)
	if errors.Is(err, sql.ErrNoRows) {
		return Product{}, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "商品不存在")
	}
	return p, err
}

func (r SQLRepository) validateTarget(ctx context.Context, in Input) error {
	if in.ProductType != "book" && in.ProductType != "chapter" {
		return nil
	}
	target, _ := strconv.ParseInt(in.TargetID, 10, 64)
	table := "novel_books"
	if in.ProductType == "chapter" {
		table = "novel_chapters"
	}
	var exists bool
	if err := r.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM `+table+` WHERE id=$1 AND deleted_at IS NULL)`, target).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "商品目标不存在")
	}
	return nil
}

func (r SQLRepository) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串")
	}
	var used bool
	if err := r.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reader_purchase_orders WHERE product_id=$1)`, id).Scan(&used); err != nil {
		return err
	}
	if used {
		return apperror.New(apperror.CodeConflict, http.StatusConflict, "已有消费订单引用该商品，请改为停售")
	}
	res, err := r.DB.ExecContext(ctx, `DELETE FROM commerce_products WHERE id=$1`, id)
	if err != nil {
		return err
	}
	count, _ := res.RowsAffected()
	if count == 0 {
		return apperror.New(apperror.CodeNotFound, http.StatusNotFound, "商品不存在")
	}
	return nil
}
