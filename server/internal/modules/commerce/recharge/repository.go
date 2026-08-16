package recharge

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/epusdt"
)

var ErrInvalidRecharge = errors.New("invalid recharge request")
var ErrRechargeProductUnavailable = errors.New("recharge product unavailable")
var ErrPaymentChannelUnavailable = errors.New("payment channel unavailable")

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

func (r SQLRepository) Prepare(ctx context.Context, req CreateRequest) (PreparedOrder, error) {
	if req.ReaderID <= 0 || req.RequestID == "" || (req.ProductID == nil && req.CustomDiamondAmount == "") || (req.ProductID != nil && req.CustomDiamondAmount != "") {
		return PreparedOrder{}, ErrInvalidRecharge
	}
	var diamonds int64
	var price, source string
	var productID *int64
	var err error
	if req.ProductID != nil {
		source = "preset"
		value := *req.ProductID
		productID = &value
		if err = r.DB.QueryRowContext(ctx, `SELECT diamond_amount,price_usdt::text FROM reader_recharge_products WHERE id=$1 AND sale_status='on_sale'`, *req.ProductID).Scan(&diamonds, &price); err != nil {
			return PreparedOrder{}, ErrRechargeProductUnavailable
		}
	} else {
		source = "custom"
		if req.CustomDiamondAmount == "" || strings.Trim(req.CustomDiamondAmount, "0123456789") != "" {
			return PreparedOrder{}, ErrInvalidRecharge
		}
		diamonds, err = strconv.ParseInt(req.CustomDiamondAmount, 10, 64)
		if err != nil {
			return PreparedOrder{}, ErrInvalidRecharge
		}
		q, e := r.Quote(ctx, diamonds)
		if e != nil {
			return PreparedOrder{}, e
		}
		price = q.PriceUSDT
	}
	var provider, currency, token, network string
	if err = r.DB.QueryRowContext(ctx, `SELECT provider,currency,token,network FROM reader_payment_channels WHERE provider='epusdt' AND enabled=true`).Scan(&provider, &currency, &token, &network); err != nil {
		return PreparedOrder{}, ErrPaymentChannelUnavailable
	}
	if provider != "epusdt" || currency != "usd" || token != "usdt" || network != "tron" {
		return PreparedOrder{}, ErrPaymentChannelUnavailable
	}
	return PreparedOrder{
		ReaderID: req.ReaderID, RequestID: req.RequestID, SourceType: source, ProductID: productID, DiamondAmount: diamonds,
		PriceUSDT: price, Provider: provider, Currency: currency, Token: token, Network: network,
	}, nil
}

func (r SQLRepository) Start(ctx context.Context, prepared PreparedOrder) (result StartResult, err error) {
	if err := validatePreparedOrder(prepared); err != nil {
		return StartResult{}, err
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return StartResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, prepared.ReaderID); err != nil {
		return StartResult{}, err
	}
	if order, found, queryErr := findOrder(tx, ctx, ` WHERE o.reader_id=$1 AND o.request_id=$2 FOR UPDATE`, prepared.ReaderID, prepared.RequestID); queryErr != nil {
		return StartResult{}, queryErr
	} else if found {
		if err = tx.Commit(); err != nil {
			return StartResult{}, err
		}
		return StartResult{Order: order}, nil
	}
	if _, err = tx.ExecContext(ctx, `UPDATE reader_recharge_orders SET status='expired',active_reader_id=NULL,failure_code='ORDER_EXPIRED',failure_message='Payment order expired',updated_at=now() WHERE active_reader_id=$1 AND status IN ('creating','pending','gateway_unknown') AND expire_time IS NOT NULL AND expire_time<=now()`, prepared.ReaderID); err != nil {
		return StartResult{}, err
	}
	active, found, err := findOrder(tx, ctx, ` WHERE o.active_reader_id=$1 FOR UPDATE`, prepared.ReaderID)
	if err != nil {
		return StartResult{}, err
	}
	if found && (active.Status == "creating" || active.Status == "gateway_unknown") {
		if err = tx.Commit(); err != nil {
			return StartResult{}, err
		}
		return StartResult{Order: active}, nil
	}
	if found {
		if active.Status != "pending" {
			return StartResult{}, fmt.Errorf("unexpected active recharge order status %q", active.Status)
		}
		if _, err = tx.ExecContext(ctx, `UPDATE reader_recharge_orders SET status='superseded',active_reader_id=NULL,failure_code='ORDER_REPLACED',failure_message='Replaced by a newer payment order',updated_at=now() WHERE id=$1`, active.ID); err != nil {
			return StartResult{}, err
		}
	}

	var id int64
	if err = tx.QueryRowContext(ctx, `SELECT nextval(pg_get_serial_sequence('reader_recharge_orders','id'))`).Scan(&id); err != nil {
		return StartResult{}, err
	}
	orderNo := "RC" + strconv.FormatInt(id, 10)
	if len(orderNo) > 32 {
		return StartResult{}, errors.New("recharge order number exceeds gateway limit")
	}
	var insertedID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO reader_recharge_orders(id,order_no,reader_id,request_id,source_type,product_id,diamond_amount,price_usdt,provider,currency,token,network,status,credential_ref,merchant_pid_snapshot,active_reader_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'creating',$13,$14,$3) ON CONFLICT DO NOTHING RETURNING id`,
		id, orderNo, prepared.ReaderID, prepared.RequestID, prepared.SourceType, prepared.ProductID, prepared.DiamondAmount, prepared.PriceUSDT,
		prepared.Provider, prepared.Currency, prepared.Token, prepared.Network, prepared.CredentialRef, prepared.MerchantPIDSnapshot).Scan(&insertedID)
	if err == sql.ErrNoRows {
		if order, found, queryErr := findOrder(tx, ctx, ` WHERE o.reader_id=$1 AND o.request_id=$2 FOR UPDATE`, prepared.ReaderID, prepared.RequestID); queryErr != nil {
			return StartResult{}, queryErr
		} else if found {
			if err = tx.Commit(); err != nil {
				return StartResult{}, err
			}
			return StartResult{Order: order}, nil
		}
		if order, found, queryErr := findOrder(tx, ctx, ` WHERE o.active_reader_id=$1 FOR UPDATE`, prepared.ReaderID); queryErr != nil {
			return StartResult{}, queryErr
		} else if found {
			if err = tx.Commit(); err != nil {
				return StartResult{}, err
			}
			return StartResult{Order: order}, nil
		}
		return StartResult{}, errors.New("recharge order conflict could not be resolved")
	}
	if err != nil {
		return StartResult{}, err
	}
	created, found, err := findOrder(tx, ctx, ` WHERE o.id=$1`, insertedID)
	if err != nil || !found {
		if err == nil {
			err = sql.ErrNoRows
		}
		return StartResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return StartResult{}, err
	}
	return StartResult{Order: created, Created: true}, nil
}

func (r SQLRepository) Complete(ctx context.Context, orderID string, response epusdt.CreateResponse) (order Order, err error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return Order{}, err
	}
	defer func() { _ = tx.Rollback() }()
	order, found, err := findOrder(tx, ctx, ` WHERE o.id=$1::bigint FOR UPDATE`, orderID)
	if err != nil {
		return Order{}, err
	}
	if !found {
		return Order{}, sql.ErrNoRows
	}
	if order.Status == "creating" {
		if _, err = tx.ExecContext(ctx, `UPDATE reader_recharge_orders SET gateway_trade_id=$1,actual_amount=$2,receive_address=$3,payment_url=$4,gateway_status=$5,expire_time=$6,status='pending',failure_code=NULL,failure_message=NULL,updated_at=now() WHERE id=$7`, response.TradeID, response.ActualAmount, response.ReceiveAddress, response.PaymentURL, response.Status, response.ExpirationTime, orderID); err != nil {
			return Order{}, err
		}
		order, _, err = findOrder(tx, ctx, ` WHERE o.id=$1::bigint`, orderID)
		if err != nil {
			return Order{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return Order{}, err
	}
	return order, nil
}

func (r SQLRepository) Fail(ctx context.Context, orderID string, failure *epusdt.GatewayError) (order Order, err error) {
	if failure == nil {
		failure = &epusdt.GatewayError{Code: "GATEWAY_CALL_FAILED", Class: epusdt.FailureUncertain, Summary: "EPUSDT create result could not be determined"}
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return Order{}, err
	}
	defer func() { _ = tx.Rollback() }()
	order, found, err := findOrder(tx, ctx, ` WHERE o.id=$1::bigint FOR UPDATE`, orderID)
	if err != nil {
		return Order{}, err
	}
	if !found {
		return Order{}, sql.ErrNoRows
	}
	if order.Status == "creating" {
		status := "create_failed"
		var activeReaderID any
		if failure.Uncertain() {
			status = "gateway_unknown"
			activeReaderID = order.ReaderID
		}
		if _, err = tx.ExecContext(ctx, `UPDATE reader_recharge_orders SET status=$1,active_reader_id=$2,failure_code=$3,failure_message=$4,updated_at=now() WHERE id=$5`, status, activeReaderID, failure.Code, failure.Summary, orderID); err != nil {
			return Order{}, err
		}
		order, _, err = findOrder(tx, ctx, ` WHERE o.id=$1::bigint`, orderID)
		if err != nil {
			return Order{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return Order{}, err
	}
	return order, nil
}

const orderSelect = `SELECT o.id,o.reader_id,o.diamond_amount::text,COALESCE(o.product_id::text,''),o.order_no,o.source_type,o.price_usdt::text,o.provider,o.currency,o.token,o.network,o.gateway_trade_id,o.actual_amount::text,o.receive_address,o.payment_url,o.block_transaction_id,o.status,o.gateway_status,o.wallet_ledger_id::text,o.expire_time,o.paid_time,o.failure_code,o.failure_message,o.created_at,o.updated_at FROM reader_recharge_orders o`

func (r SQLRepository) GetOrder(ctx context.Context, readerID int64, orderID string) (Order, error) {
	var o Order
	err := scanOrder(r.DB.QueryRowContext(ctx, orderSelect+` WHERE o.reader_id=$1 AND o.id=$2::bigint`, readerID, orderID), &o)
	return o, err
}
func scanOrder(s interface{ Scan(...any) error }, o *Order) error {
	return s.Scan(&o.ID, &o.ReaderID, &o.DiamondAmount, &o.ProductID, &o.OrderNo, &o.SourceType, &o.PriceUSDT, &o.Provider, &o.Currency, &o.Token, &o.Network, &o.GatewayTradeID, &o.ActualAmount, &o.ReceiveAddress, &o.PaymentURL, &o.BlockTransactionID, &o.Status, &o.GatewayStatus, &o.WalletLedgerID, &o.ExpireTime, &o.PaidTime, &o.FailureCode, &o.FailureMessage, &o.CreateTime, &o.UpdateTime)
}

type orderQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func findOrder(queryer orderQueryer, ctx context.Context, suffix string, args ...any) (Order, bool, error) {
	var order Order
	err := scanOrder(queryer.QueryRowContext(ctx, orderSelect+suffix, args...), &order)
	if err == sql.ErrNoRows {
		return Order{}, false, nil
	}
	return order, err == nil, err
}

func validatePreparedOrder(prepared PreparedOrder) error {
	if prepared.ReaderID <= 0 || strings.TrimSpace(prepared.RequestID) == "" || prepared.DiamondAmount <= 0 || prepared.PriceUSDT == "" ||
		(prepared.SourceType != "preset" && prepared.SourceType != "custom") || prepared.Provider != "epusdt" || prepared.Currency != "usd" ||
		prepared.Token != "usdt" || prepared.Network != "tron" || strings.TrimSpace(prepared.CredentialRef) == "" || strings.TrimSpace(prepared.MerchantPIDSnapshot) == "" {
		return ErrInvalidRecharge
	}
	return nil
}
