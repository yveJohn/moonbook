package recharge

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"github.com/gin-gonic/gin"
)

const (
	rechargeMaxID  int64 = 9223372036854775807
	rechargeSafeID int64 = 9007199254740993
)

type rechargeContractRepo struct {
	err          error
	quotedAmount int64
	create       CreateRequest
	getReaderID  int64
	getOrderID   string
}

func (r *rechargeContractRepo) Catalog(context.Context) (Catalog, error) {
	if r.err != nil {
		return Catalog{}, r.err
	}
	return Catalog{
		Products: []Product{{
			ID: rechargeMaxID, DiamondAmount: rechargeSafeID, ProductName: "边界充值",
			PriceUSDT: "123456789.12345678", SaleStatus: "on_sale", SortOrder: 1,
		}},
		CustomEnabled: true, DiamondsPerUSDT: "100.00000000",
		MinDiamondAmount: "1", MaxDiamondAmount: "9223372036854775807",
	}, nil
}

func (r *rechargeContractRepo) Quote(_ context.Context, amount int64) (Quote, error) {
	if r.err != nil {
		return Quote{}, r.err
	}
	r.quotedAmount = amount
	return Quote{DiamondAmount: "9223372036854775807", PriceUSDT: "92233720368.54775807"}, nil
}

func (r *rechargeContractRepo) CreateOrder(_ context.Context, request CreateRequest) (Order, error) {
	if r.err != nil {
		return Order{}, r.err
	}
	r.create = request
	return rechargeContractOrder("9223372036854775807", "9007199254740993"), nil
}

func (r *rechargeContractRepo) GetOrder(_ context.Context, readerID int64, orderID string) (Order, error) {
	if r.err != nil {
		return Order{}, r.err
	}
	r.getReaderID, r.getOrderID = readerID, orderID
	return rechargeContractOrder(orderID, ""), nil
}

func rechargeContractOrder(id, productID string) Order {
	gatewayTradeID, receiveAddress := "gateway-contract", "T-contract"
	walletLedgerID := "9223372036854775806"
	expireTime := time.Date(2026, 8, 15, 3, 14, 5, 0, time.UTC)
	createdAt := time.Date(2026, 8, 15, 3, 4, 5, 0, time.UTC)
	return Order{
		ID: id, ReaderID: "9223372036854775807", OrderNo: "R-contract", SourceType: "preset",
		ProductID: productID, DiamondAmount: "9007199254740993", PriceUSDT: "123456789.12345678",
		Provider: "epusdt", Currency: "USDT", Token: "USDT", Network: "TRC20",
		GatewayTradeID: &gatewayTradeID, ReceiveAddress: &receiveAddress, Status: "pending",
		WalletLedgerID: &walletLedgerID, ExpireTime: &expireTime, CreateTime: createdAt, UpdateTime: createdAt,
	}
}

func TestRechargeHTTPContractKeepsIDsAmountsAndNulls(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &rechargeContractRepo{}
	handler := &Handler{service: NewService(repo)}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("reader.identity", readerauth.Identity{ReaderID: rechargeMaxID, SessionID: rechargeSafeID})
		c.Next()
	})
	router.GET("/reader/recharge/products", handler.catalog)
	router.POST("/reader/recharge/quote", handler.quote)
	router.POST("/reader/me/recharge/orders", handler.create)
	router.GET("/reader/me/recharge/orders/:orderId", handler.get)

	tests := []struct {
		name, method, path, body, want string
	}{
		{
			"catalog", http.MethodGet, "/reader/recharge/products", "",
			`{"code":200,"msg":"查询成功","data":{"products":[{"id":"9223372036854775807","productName":"边界充值","diamondAmount":"9007199254740993","priceUsdt":"123456789.12345678","saleStatus":"on_sale","sortOrder":1}],"customEnabled":true,"diamondsPerUsdt":"100.00000000","minDiamondAmount":"1","maxDiamondAmount":"9223372036854775807"}}`,
		},
		{
			"quote", http.MethodPost, "/reader/recharge/quote", `{"diamondAmount":"9223372036854775807"}`,
			`{"code":200,"msg":"查询成功","data":{"diamondAmount":"9223372036854775807","priceUsdt":"92233720368.54775807"}}`,
		},
		{
			"create preset order", http.MethodPost, "/reader/me/recharge/orders", `{"productId":"9007199254740993","requestId":"request-contract"}`,
			`{"code":200,"msg":"订单创建成功","data":{"id":"9223372036854775807","orderNo":"R-contract","readerId":"9223372036854775807","sourceType":"preset","productId":"9007199254740993","diamondAmount":"9007199254740993","priceUsdt":"123456789.12345678","provider":"epusdt","currency":"USDT","token":"USDT","network":"TRC20","gatewayTradeId":"gateway-contract","actualAmount":null,"receiveAddress":"T-contract","paymentUrl":null,"blockTransactionId":null,"status":"pending","gatewayStatus":null,"walletLedgerId":"9223372036854775806","expireTime":"2026-08-15 03:14:05","paidTime":null,"failureCode":null,"failureMessage":null,"createTime":"2026-08-15 03:04:05","updateTime":"2026-08-15 03:04:05"}}`,
		},
		{
			"get custom order", http.MethodGet, "/reader/me/recharge/orders/9223372036854775807", "",
			`{"code":200,"msg":"查询成功","data":{"id":"9223372036854775807","orderNo":"R-contract","readerId":"9223372036854775807","sourceType":"preset","productId":null,"diamondAmount":"9007199254740993","priceUsdt":"123456789.12345678","provider":"epusdt","currency":"USDT","token":"USDT","network":"TRC20","gatewayTradeId":"gateway-contract","actualAmount":null,"receiveAddress":"T-contract","paymentUrl":null,"blockTransactionId":null,"status":"pending","gatewayStatus":null,"walletLedgerId":"9223372036854775806","expireTime":"2026-08-15 03:14:05","paidTime":null,"failureCode":null,"failureMessage":null,"createTime":"2026-08-15 03:04:05","updateTime":"2026-08-15 03:04:05"}}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, bytes.NewBufferString(test.body))
			if test.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
			}
			assertRechargeJSON(t, test.want, resp.Body.String())
		})
	}
	if repo.quotedAmount != rechargeMaxID {
		t.Fatalf("quoted amount=%d", repo.quotedAmount)
	}
	if repo.create.ReaderID != rechargeMaxID || repo.create.ProductID == nil || *repo.create.ProductID != rechargeSafeID || repo.create.RequestID != "request-contract" {
		t.Fatalf("create request=%+v", repo.create)
	}
	if repo.getReaderID != rechargeMaxID || repo.getOrderID != "9223372036854775807" {
		t.Fatalf("get reader=%d order=%s", repo.getReaderID, repo.getOrderID)
	}
}

func TestRechargeHTTPErrorContracts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name, method, path, body, want string
		err                            error
	}{
		{name: "invalid quote", method: http.MethodPost, path: "/reader/recharge/quote", body: `{"diamondAmount":"not-a-number"}`, want: `{"code":500,"msg":"充值参数无效","data":null}`},
		{name: "product unavailable", method: http.MethodPost, path: "/reader/me/recharge/orders", body: `{"productId":"9007199254740993","requestId":"request-error"}`, err: ErrRechargeProductUnavailable, want: `{"code":500,"msg":"充值档位不存在或已下架","data":null}`},
		{name: "order not found", method: http.MethodGet, path: "/reader/me/recharge/orders/9007199254740993", err: sql.ErrNoRows, want: `{"code":500,"msg":"充值订单不存在","data":null}`},
		{name: "repository failure", method: http.MethodGet, path: "/reader/recharge/products", err: errors.New("database host secret detail"), want: `{"code":500,"msg":"internal service error","data":null}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &rechargeContractRepo{err: test.err}
			handler := &Handler{service: NewService(repo)}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("reader.identity", readerauth.Identity{ReaderID: rechargeMaxID, SessionID: rechargeSafeID})
				c.Next()
			})
			router.GET("/reader/recharge/products", handler.catalog)
			router.POST("/reader/recharge/quote", handler.quote)
			router.POST("/reader/me/recharge/orders", handler.create)
			router.GET("/reader/me/recharge/orders/:orderId", handler.get)
			req := httptest.NewRequest(test.method, test.path, bytes.NewBufferString(test.body))
			if test.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
			}
			assertRechargeJSON(t, test.want, resp.Body.String())
		})
	}
}

func assertRechargeJSON(t *testing.T, want, got string) {
	t.Helper()
	var wantValue, gotValue any
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(got), &gotValue); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, got)
	}
	if !reflect.DeepEqual(wantValue, gotValue) {
		t.Fatalf("response mismatch\nwant: %s\n got: %s", want, got)
	}
}
