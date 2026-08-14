package recharge

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidRecharge = errors.New("invalid recharge request")
var ErrRechargeProductUnavailable = errors.New("recharge product unavailable")

type SQLRepository struct{ DB *sql.DB }

func (r SQLRepository) Catalog(ctx context.Context) (Catalog, error) {
	var c Catalog
	var rate, min, max string
	if err := r.DB.QueryRowContext(ctx, `SELECT custom_enabled,diamonds_per_usdt::text,min_diamond_amount::text,max_diamond_amount::text FROM reader_recharge_settings WHERE id=1`).Scan(&c.CustomEnabled, &rate, &min, &max); err != nil {
		return c, err
	}
	c.DiamondsPerUSDT, c.MinDiamondAmount, c.MaxDiamondAmount = rate, min, max
	rows, err := r.DB.QueryContext(ctx, `SELECT id,product_name,diamond_amount,price_usdt::text,sale_status,sort_order FROM reader_recharge_products WHERE sale_status='on_sale' ORDER BY sort_order,id`)
	if err != nil {
		return c, err
	}
	defer rows.Close()
	c.Products = []Product{}
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.ProductName, &p.DiamondAmount, &p.PriceUSDT, &p.SaleStatus, &p.SortOrder); err != nil {
			return c, err
		}
		c.Products = append(c.Products, p)
	}
	return c, rows.Err()
}

func (r SQLRepository) Quote(ctx context.Context, diamonds int64) (Quote, error) {
	var rate string
	var min, max int64
	if err := r.DB.QueryRowContext(ctx, `SELECT diamonds_per_usdt::text,min_diamond_amount,max_diamond_amount FROM reader_recharge_settings WHERE id=1`).Scan(&rate, &min, &max); err != nil {
		return Quote{}, err
	}
	if diamonds < min || diamonds > max {
		return Quote{}, ErrInvalidRecharge
	}
	price, err := ceilMoney(diamonds, rate)
	if err != nil {
		return Quote{}, ErrInvalidRecharge
	}
	return Quote{DiamondAmount: strconv.FormatInt(diamonds, 10), PriceUSDT: price}, nil
}

func ceilMoney(diamonds int64, rate string) (string, error) {
	r := new(big.Rat)
	if _, ok := r.SetString(strings.TrimSpace(rate)); !ok || r.Sign() <= 0 {
		return "", ErrInvalidRecharge
	}
	numerator := new(big.Int).Mul(big.NewInt(diamonds), big.NewInt(100))
	numerator.Mul(numerator, r.Denom())
	cents := new(big.Int).Quo(new(big.Int).Set(numerator), r.Num())
	if new(big.Int).Mod(numerator, r.Num()).Sign() != 0 {
		cents.Add(cents, big.NewInt(1))
	}
	return fmt.Sprintf("%s.%02d", new(big.Int).Quo(cents, big.NewInt(100)), new(big.Int).Mod(cents, big.NewInt(100))), nil
}

func (r SQLRepository) CreateOrder(ctx context.Context, req CreateRequest) (Order, error) {
	if req.ReaderID <= 0 || req.RequestID == "" || (req.ProductID == nil && req.CustomDiamondAmount == "") || (req.ProductID != nil && req.CustomDiamondAmount != "") {
		return Order{}, ErrInvalidRecharge
	}
	var existing Order
	err := scanOrder(r.DB.QueryRowContext(ctx, orderSelect+` WHERE o.reader_id=$1 AND o.request_id=$2`, req.ReaderID, req.RequestID), &existing)
	if err == nil {
		return existing, nil
	}
	if err != sql.ErrNoRows {
		return Order{}, err
	}
	var diamonds int64
	var price, source string
	var productID any
	if req.ProductID != nil {
		source = "preset"
		productID = *req.ProductID
		if err = r.DB.QueryRowContext(ctx, `SELECT diamond_amount,price_usdt::text FROM reader_recharge_products WHERE id=$1 AND sale_status='on_sale'`, *req.ProductID).Scan(&diamonds, &price); err != nil {
			return Order{}, ErrRechargeProductUnavailable
		}
	} else {
		source = "custom"
		if req.CustomDiamondAmount == "" || strings.Trim(req.CustomDiamondAmount, "0123456789") != "" {
			return Order{}, ErrInvalidRecharge
		}
		diamonds, err = strconv.ParseInt(req.CustomDiamondAmount, 10, 64)
		if err != nil {
			return Order{}, ErrInvalidRecharge
		}
		q, e := r.Quote(ctx, diamonds)
		if e != nil {
			return Order{}, e
		}
		price = q.PriceUSDT
	}
	var provider, currency, token, network string
	if err = r.DB.QueryRowContext(ctx, `SELECT provider,currency,token,network FROM reader_payment_channels WHERE enabled=true ORDER BY id LIMIT 1`).Scan(&provider, &currency, &token, &network); err != nil {
		return Order{}, errors.New("payment channel unavailable")
	}
	orderNo := fmt.Sprintf("MBR%d", time.Now().UnixNano())
	var id int64
	err = r.DB.QueryRowContext(ctx, `INSERT INTO reader_recharge_orders(order_no,reader_id,request_id,source_type,product_id,diamond_amount,price_usdt,provider,currency,token,network,status,expire_time) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'pending',now()+interval '30 minutes') RETURNING id`, orderNo, req.ReaderID, req.RequestID, source, productID, diamonds, price, provider, currency, token, network).Scan(&id)
	if err != nil {
		return Order{}, err
	}
	return r.GetOrder(ctx, req.ReaderID, strconv.FormatInt(id, 10))
}

const orderSelect = `SELECT o.id,o.reader_id,o.diamond_amount::text,COALESCE(o.product_id::text,''),o.order_no,o.source_type,o.price_usdt::text,o.provider,o.currency,o.token,o.network,o.gateway_trade_id,o.actual_amount::text,o.receive_address,o.payment_url,o.block_transaction_id,o.status,o.gateway_status,o.wallet_ledger_id::text,o.expire_time::text,o.paid_time::text,o.failure_code,o.failure_message,o.created_at,o.updated_at FROM reader_recharge_orders o`

func (r SQLRepository) GetOrder(ctx context.Context, readerID int64, orderID string) (Order, error) {
	var o Order
	err := scanOrder(r.DB.QueryRowContext(ctx, orderSelect+` WHERE o.reader_id=$1 AND o.id=$2::bigint`, readerID, orderID), &o)
	return o, err
}
func scanOrder(s interface{ Scan(...any) error }, o *Order) error {
	return s.Scan(&o.ID, &o.ReaderID, &o.DiamondAmount, &o.ProductID, &o.OrderNo, &o.SourceType, &o.PriceUSDT, &o.Provider, &o.Currency, &o.Token, &o.Network, &o.GatewayTradeID, &o.ActualAmount, &o.ReceiveAddress, &o.PaymentURL, &o.BlockTransactionID, &o.Status, &o.GatewayStatus, &o.WalletLedgerID, &o.ExpireTime, &o.PaidTime, &o.FailureCode, &o.FailureMessage, &o.CreateTime, &o.UpdateTime)
}
