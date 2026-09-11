package commercecompat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"github.com/gin-gonic/gin"
)

const (
	contractMaxID  int64 = 9223372036854775807
	contractSafeID int64 = 9007199254740993
)

func contractRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		ctx.Set("reader.identity", readerauth.Identity{ReaderID: contractMaxID, SessionID: contractSafeID})
		ctx.Next()
	})
	return router
}

func assertContractJSON(t *testing.T, want, got string) {
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

type checkinContract struct{ statusID, checkinID int64 }

func (stub *checkinContract) CheckinStatus(_ context.Context, readerID int64) (commercecontract.CheckinStatus, error) {
	stub.statusID = readerID
	return commercecontract.CheckinStatus{TodayRewardCoin: contractSafeID, RewardRandom: true, CheckinAvailable: true}, nil
}

func (stub *checkinContract) Checkin(_ context.Context, readerID int64) (commercecontract.CheckinStatus, error) {
	stub.checkinID = readerID
	return commercecontract.CheckinStatus{TodayChecked: true, ContinuousDays: 7, TodayRewardCoin: contractMaxID, RewardText: "获得边界奖励", UnavailableReason: "今日已签到"}, nil
}

func TestCheckinHTTPContractKeepsRewardAsStringAndNulls(t *testing.T) {
	stub := &checkinContract{}
	router := contractRouter()
	handler := checkinHandler{service: stub}
	router.GET("/reader/me/checkin/status", handler.status)
	router.POST("/reader/me/checkin", handler.checkin)

	tests := []struct{ name, method, path, want string }{
		{"status", http.MethodGet, "/reader/me/checkin/status", `{"code":200,"msg":"查询成功","data":{"todayChecked":false,"continuousDays":0,"todayRewardCoin":"9007199254740993","rewardRandom":true,"rewardText":null,"checkinAvailable":true,"unavailableReason":null}}`},
		{"checkin", http.MethodPost, "/reader/me/checkin", `{"code":200,"msg":"签到成功","data":{"todayChecked":true,"continuousDays":7,"todayRewardCoin":"9223372036854775807","rewardRandom":false,"rewardText":"获得边界奖励","checkinAvailable":false,"unavailableReason":"今日已签到"}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			assertContractJSON(t, test.want, response.Body.String())
		})
	}
	if stub.statusID != contractMaxID || stub.checkinID != contractMaxID {
		t.Fatalf("status reader=%d checkin reader=%d", stub.statusID, stub.checkinID)
	}
}

type walletContract struct {
	coinType   string
	page, size int
}

func (*walletContract) Wallet(context.Context, int64) (commercecontract.Wallet, error) {
	return commercecontract.Wallet{ReaderID: contractMaxID, RechargeCoinBalance: contractMaxID, BonusCoinBalance: contractSafeID, TotalRechargeCoinIncome: contractMaxID - 1, TotalBonusCoinIncome: contractSafeID + 1, TotalRechargeCoinExpense: contractMaxID - 2, TotalBonusCoinExpense: contractSafeID + 2}, nil
}
func (*walletContract) Wallets(context.Context, []int64) ([]commercecontract.Wallet, error) {
	return nil, nil
}

func (stub *walletContract) WalletLedgers(_ context.Context, readerID int64, coinType string, page, size int) ([]commercecontract.WalletLedger, int64, error) {
	stub.coinType, stub.page, stub.size = coinType, page, size
	bizID, remark := "9223372036854775806", "契约流水"
	return []commercecontract.WalletLedger{{ID: contractMaxID, ReaderID: readerID, Amount: contractSafeID, BalanceBefore: contractMaxID - 1, BalanceAfter: contractMaxID, LedgerNo: "L-contract", BizType: "recharge", Direction: "income", CoinType: coinType, BizID: &bizID, Remark: &remark, CreatedAt: time.Date(2026, 8, 15, 2, 3, 4, 0, time.UTC)}}, 1, nil
}

func TestWalletHTTPContractKeepsIDsAndAmountsAsStrings(t *testing.T) {
	stub := &walletContract{}
	router := contractRouter()
	handler := walletHandler{service: stub}
	router.GET("/reader/me/wallet", handler.get)
	router.GET("/reader/me/wallet/ledgers", handler.ledgers)
	tests := []struct{ name, path, want string }{
		{"wallet", "/reader/me/wallet", `{"code":200,"msg":"查询成功","data":{"readerId":"9223372036854775807","rechargeCoinBalance":"9223372036854775807","bonusCoinBalance":"9007199254740993","totalRechargeCoinIncome":"9223372036854775806","totalBonusCoinIncome":"9007199254740994","totalRechargeCoinExpense":"9223372036854775805","totalBonusCoinExpense":"9007199254740995","expiringBonusCoin":"0"}}`},
		{"ledger page", "/reader/me/wallet/ledgers?coinType=bonus&pageNum=0&pageSize=1000", `{"code":200,"msg":"查询成功","rows":[{"id":"9223372036854775807","readerId":"9223372036854775807","ledgerNo":"L-contract","bizType":"recharge","bizId":"9223372036854775806","orderNo":null,"direction":"income","coinType":"bonus","amount":"9007199254740993","balanceBefore":"9223372036854775806","balanceAfter":"9223372036854775807","remark":"契约流水","createTime":"2026-08-15 02:03:04"}],"total":1}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			assertContractJSON(t, test.want, response.Body.String())
		})
	}
	if stub.coinType != "bonus" || stub.page != 1 || stub.size != 100 {
		t.Fatalf("ledger contract query coin=%s page=%d size=%d", stub.coinType, stub.page, stub.size)
	}
}

type purchaseContract struct {
	err                                    error
	membershipReader, membershipProduct    int64
	membershipRequest                      string
	chapterReader, chapterID, chapterPrice int64
	chapterRequest                         string
	bookReader, bookID, bookPrice          int64
}

func contractPurchaseOrder(orderType string, productID, targetID, bookID *int64) commercecontract.PurchaseOrder {
	remark := "契约订单"
	paidAt := time.Date(2026, 8, 15, 4, 5, 6, 0, time.UTC)
	wordCount, wordUnit, coinUnit := 2001, 1000, contractSafeID
	return commercecontract.PurchaseOrder{ID: contractMaxID, ReaderID: contractMaxID, OrderNo: "O-contract", OrderType: orderType, ProductID: productID, ProductType: orderType, TargetID: targetID, BookIDSnapshot: bookID, ProductName: "契约商品", PriceCoin: contractMaxID, ChapterWordCount: &wordCount, PricingWordUnit: &wordUnit, PricingCoinUnit: &coinUnit, RechargeCoinAmount: contractSafeID, BonusCoinAmount: contractMaxID - 1, Status: "paid", IdempotencyKey: orderType + ":contract", Remark: &remark, PaidAt: &paidAt, CreatedAt: paidAt, UpdatedAt: paidAt}
}

func (stub *purchaseContract) BuyMembership(_ context.Context, readerID, productID int64, request string) (commercecontract.PurchaseOrder, error) {
	if stub.err != nil {
		return commercecontract.PurchaseOrder{}, stub.err
	}
	stub.membershipReader, stub.membershipProduct, stub.membershipRequest = readerID, productID, request
	return contractPurchaseOrder("membership", &productID, nil, nil), nil
}

func (stub *purchaseContract) BuyChapter(_ context.Context, readerID, chapterID, expectedPrice int64, request string) (commercecontract.ChapterPurchaseResult, error) {
	if stub.err != nil {
		return commercecontract.ChapterPurchaseResult{}, stub.err
	}
	stub.chapterReader, stub.chapterID, stub.chapterPrice, stub.chapterRequest = readerID, chapterID, expectedPrice, request
	bookID := contractSafeID + 1
	order := contractPurchaseOrder("chapter", nil, &chapterID, &bookID)
	return commercecontract.ChapterPurchaseResult{Status: "paid", Quote: commercecontract.ChapterQuote{ChapterID: chapterID, BookID: bookID, WordCount: 2001, WordUnit: 1000, CoinUnit: contractSafeID, PriceCoin: expectedPrice}, Order: &order}, nil
}

func (stub *purchaseContract) BuyBook(_ context.Context, readerID, bookID, expectedPrice int64) (commercecontract.PurchaseOrder, error) {
	if stub.err != nil {
		return commercecontract.PurchaseOrder{}, stub.err
	}
	stub.bookReader, stub.bookID, stub.bookPrice = readerID, bookID, expectedPrice
	productID := contractSafeID + 2
	return contractPurchaseOrder("book", &productID, &bookID, &bookID), nil
}

func TestPurchaseHTTPContractKeepsOrderFieldsAndLongValues(t *testing.T) {
	stub := &purchaseContract{}
	router := contractRouter()
	handler := purchaseHandler{service: stub}
	router.POST("/reader/me/orders/membership", handler.membership)
	router.POST("/reader/me/orders/chapter", handler.chapter)
	router.POST("/reader/me/orders/book", handler.book)
	tests := []struct{ name, path, body, want string }{
		{"membership", "/reader/me/orders/membership", `{"productId":"9007199254740993","requestId":"membership-request"}`, `{"code":200,"msg":"购买成功","data":{"id":"9223372036854775807","orderNo":"O-contract","readerId":"9223372036854775807","orderType":"membership","productId":"9007199254740993","productType":"membership","targetId":null,"bookIdSnapshot":null,"productNameSnapshot":"契约商品","priceCoinSnapshot":"9223372036854775807","chapterWordCountSnapshot":2001,"pricingWordUnitSnapshot":1000,"pricingCoinUnitSnapshot":"9007199254740993","rechargeCoinAmount":"9007199254740993","bonusCoinAmount":"9223372036854775806","status":"paid","idempotencyKey":"membership:contract","remark":"契约订单","operatorId":null,"paidTime":"2026-08-15 04:05:06","createTime":"2026-08-15 04:05:06","updateTime":"2026-08-15 04:05:06"}}`},
		{"chapter", "/reader/me/orders/chapter", `{"chapterId":"9223372036854775807","expectedPrice":"9223372036854775807","requestId":"chapter-request"}`, `{"code":200,"msg":"购买成功","data":{"purchaseStatus":"paid","quote":{"chapterId":"9223372036854775807","bookId":"9007199254740994","wordCount":2001,"wordUnit":1000,"coinUnit":"9007199254740993","priceCoin":"9223372036854775807"},"order":{"id":"9223372036854775807","orderNo":"O-contract","readerId":"9223372036854775807","orderType":"chapter","productId":null,"productType":"chapter","targetId":"9223372036854775807","bookIdSnapshot":"9007199254740994","productNameSnapshot":"契约商品","priceCoinSnapshot":"9223372036854775807","chapterWordCountSnapshot":2001,"pricingWordUnitSnapshot":1000,"pricingCoinUnitSnapshot":"9007199254740993","rechargeCoinAmount":"9007199254740993","bonusCoinAmount":"9223372036854775806","status":"paid","idempotencyKey":"chapter:contract","remark":"契约订单","operatorId":null,"paidTime":"2026-08-15 04:05:06","createTime":"2026-08-15 04:05:06","updateTime":"2026-08-15 04:05:06"}}}`},
		{"book", "/reader/me/orders/book", `{"bookId":"9007199254740993","expectedPrice":"9223372036854775807"}`, `{"code":200,"msg":"购买成功","data":{"id":"9223372036854775807","orderNo":"O-contract","readerId":"9223372036854775807","orderType":"book","productId":"9007199254740995","productType":"book","targetId":"9007199254740993","bookIdSnapshot":"9007199254740993","productNameSnapshot":"契约商品","priceCoinSnapshot":"9223372036854775807","chapterWordCountSnapshot":2001,"pricingWordUnitSnapshot":1000,"pricingCoinUnitSnapshot":"9007199254740993","rechargeCoinAmount":"9007199254740993","bonusCoinAmount":"9223372036854775806","status":"paid","idempotencyKey":"book:contract","remark":"契约订单","operatorId":null,"paidTime":"2026-08-15 04:05:06","createTime":"2026-08-15 04:05:06","updateTime":"2026-08-15 04:05:06"}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.path, bytes.NewBufferString(test.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			assertContractJSON(t, test.want, response.Body.String())
		})
	}
	if stub.membershipReader != contractMaxID || stub.membershipProduct != contractSafeID || stub.membershipRequest != "membership-request" || stub.chapterReader != contractMaxID || stub.chapterID != contractMaxID || stub.chapterPrice != contractMaxID || stub.chapterRequest != "chapter-request" || stub.bookReader != contractMaxID || stub.bookID != contractSafeID || stub.bookPrice != contractMaxID {
		t.Fatalf("purchase calls=%+v", stub)
	}
}

func TestPurchaseHTTPErrorContracts(t *testing.T) {
	tests := []struct {
		name, body, want string
		err              error
	}{
		{"quote changed", `{"bookId":"9007199254740993","expectedPrice":"1"}`, `{"code":46106,"msg":"作品报价已变化，请确认新价格","data":null}`, commercecontract.ErrQuoteChanged},
		{"insufficient balance", `{"bookId":"9007199254740993","expectedPrice":"1"}`, `{"code":500,"msg":"余额不足","data":null}`, commercecontract.ErrInsufficientFunds},
		{"product unavailable", `{"bookId":"9007199254740993","expectedPrice":"1"}`, `{"code":200,"msg":"购买商品不可用","data":null}`, commercecontract.ErrProductUnavailable},
		{"invalid price", `{"bookId":"9007199254740993","expectedPrice":null}`, `{"code":200,"msg":"购买参数无效","data":null}`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := contractRouter()
			handler := purchaseHandler{service: &purchaseContract{err: test.err}}
			router.POST("/reader/me/orders/book", handler.book)
			request := httptest.NewRequest(http.MethodPost, "/reader/me/orders/book", bytes.NewBufferString(test.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			assertContractJSON(t, test.want, response.Body.String())
		})
	}
}

type rechargeContract struct {
	err                 error
	order               *commercecontract.RechargeOrder
	quoted              int64
	created             commercecontract.RechargeCreateRequest
	getReader, getOrder int64
}

func (stub *rechargeContract) RechargeCatalog(context.Context) (commercecontract.RechargeCatalog, error) {
	if stub.err != nil {
		return commercecontract.RechargeCatalog{}, stub.err
	}
	return commercecontract.RechargeCatalog{Products: []commercecontract.RechargeProduct{{ID: contractMaxID, Name: "边界充值", DiamondAmount: contractSafeID, PriceUSDT: "123456789.12345678", SaleStatus: "on_sale", SortOrder: 1}}, CustomEnabled: true, DiamondsPerUSDT: "100.00000000", MinDiamondAmount: "1", MaxDiamondAmount: "9223372036854775807"}, nil
}
func (stub *rechargeContract) QuoteRecharge(_ context.Context, amount int64) (commercecontract.RechargeQuote, error) {
	if stub.err != nil {
		return commercecontract.RechargeQuote{}, stub.err
	}
	stub.quoted = amount
	return commercecontract.RechargeQuote{DiamondAmount: contractMaxID, PriceUSDT: "92233720368.54775807"}, nil
}
func contractRechargeOrder(id int64, productID *int64) commercecontract.RechargeOrder {
	gateway, address := "gateway-contract", "T-contract"
	ledger := contractMaxID - 1
	expires := time.Date(2026, 8, 15, 3, 14, 5, 0, time.UTC)
	created := time.Date(2026, 8, 15, 3, 4, 5, 0, time.UTC)
	return commercecontract.RechargeOrder{ID: id, ReaderID: contractMaxID, DiamondAmount: contractSafeID, ProductID: productID, OrderNo: "R-contract", SourceType: "preset", PriceUSDT: "123456789.12345678", Provider: "epusdt", Currency: "USDT", Token: "USDT", Network: "TRC20", GatewayTradeID: &gateway, ReceiveAddress: &address, Status: "pending", WalletLedgerID: &ledger, ExpiresAt: &expires, CreatedAt: created, UpdatedAt: created}
}
func (stub *rechargeContract) CreateRechargeOrder(_ context.Context, request commercecontract.RechargeCreateRequest) (commercecontract.RechargeOrder, error) {
	if stub.err != nil {
		return commercecontract.RechargeOrder{}, stub.err
	}
	stub.created = request
	if stub.order != nil {
		return *stub.order, nil
	}
	return contractRechargeOrder(contractMaxID, request.ProductID), nil
}

func TestRechargeCreateHTTPContractKeepsGatewayFailureStatesPrivate(t *testing.T) {
	codeRejected, messageRejected := "GATEWAY_REJECTED", "EPUSDT rejected the create request"
	codeTimeout, messageTimeout := "REQUEST_TIMEOUT", "EPUSDT create request timed out"
	tests := []struct {
		name           string
		status         string
		failureCode    *string
		failureMessage *string
	}{
		{name: "success", status: "pending"},
		{name: "definite failure", status: "create_failed", failureCode: &codeRejected, failureMessage: &messageRejected},
		{name: "uncertain result", status: "gateway_unknown", failureCode: &codeTimeout, failureMessage: &messageTimeout},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			order := contractRechargeOrder(contractMaxID, nil)
			order.Status = test.status
			order.FailureCode = test.failureCode
			order.FailureMessage = test.failureMessage
			if test.status != "pending" {
				order.GatewayTradeID = nil
				order.ReceiveAddress = nil
				order.ExpiresAt = nil
			}
			router := contractRouter()
			handler := rechargeHandler{service: &rechargeContract{order: &order}}
			router.POST("/reader/me/recharge/orders", handler.create)
			request := httptest.NewRequest(http.MethodPost, "/reader/me/recharge/orders", bytes.NewBufferString(`{"customDiamondAmount":"100","requestId":"state-contract"}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			var body struct {
				Code int            `json:"code"`
				Msg  string         `json:"msg"`
				Data map[string]any `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Code != 200 || body.Msg != "订单创建成功" || body.Data["status"] != test.status || body.Data["id"] != "9223372036854775807" {
				t.Fatalf("body=%s", response.Body.String())
			}
			wantCode, wantMessage := any(nil), any(nil)
			if test.failureCode != nil {
				wantCode = *test.failureCode
				wantMessage = *test.failureMessage
			}
			if body.Data["failureCode"] != wantCode || body.Data["failureMessage"] != wantMessage {
				t.Fatalf("body=%s", response.Body.String())
			}
			for _, privateField := range []string{"credentialRef", "merchantPidSnapshot", "secret", "signature"} {
				if _, exists := body.Data[privateField]; exists {
					t.Fatalf("private field %q leaked: %s", privateField, response.Body.String())
				}
			}
		})
	}
}
func (stub *rechargeContract) RechargeOrder(_ context.Context, readerID, orderID int64) (commercecontract.RechargeOrder, error) {
	if stub.err != nil {
		return commercecontract.RechargeOrder{}, stub.err
	}
	stub.getReader, stub.getOrder = readerID, orderID
	return contractRechargeOrder(orderID, nil), nil
}

func TestRechargeHTTPContractKeepsIDsAmountsAndNulls(t *testing.T) {
	stub := &rechargeContract{}
	router := contractRouter()
	handler := rechargeHandler{service: stub}
	router.GET("/reader/recharge/products", handler.catalog)
	router.POST("/reader/recharge/quote", handler.quote)
	router.POST("/reader/me/recharge/orders", handler.create)
	router.GET("/reader/me/recharge/orders/:orderId", handler.get)
	tests := []struct{ name, method, path, body, want string }{
		{"catalog", http.MethodGet, "/reader/recharge/products", "", `{"code":200,"msg":"查询成功","data":{"products":[{"id":"9223372036854775807","productName":"边界充值","diamondAmount":"9007199254740993","priceUsdt":"123456789.12345678","saleStatus":"on_sale","sortOrder":1}],"customEnabled":true,"diamondsPerUsdt":"100.00000000","minDiamondAmount":"1","maxDiamondAmount":"9223372036854775807"}}`},
		{"quote", http.MethodPost, "/reader/recharge/quote", `{"diamondAmount":"9223372036854775807"}`, `{"code":200,"msg":"查询成功","data":{"diamondAmount":"9223372036854775807","priceUsdt":"92233720368.54775807"}}`},
		{"create preset order", http.MethodPost, "/reader/me/recharge/orders", `{"productId":"9007199254740993","requestId":"request-contract"}`, `{"code":200,"msg":"订单创建成功","data":{"id":"9223372036854775807","orderNo":"R-contract","readerId":"9223372036854775807","sourceType":"preset","productId":"9007199254740993","diamondAmount":"9007199254740993","priceUsdt":"123456789.12345678","provider":"epusdt","currency":"USDT","token":"USDT","network":"TRC20","gatewayTradeId":"gateway-contract","actualAmount":null,"receiveAddress":"T-contract","paymentUrl":null,"blockTransactionId":null,"status":"pending","gatewayStatus":null,"walletLedgerId":"9223372036854775806","expireTime":"2026-08-15 03:14:05","paidTime":null,"failureCode":null,"failureMessage":null,"createTime":"2026-08-15 03:04:05","updateTime":"2026-08-15 03:04:05"}}`},
		{"get custom order", http.MethodGet, "/reader/me/recharge/orders/9223372036854775807", "", `{"code":200,"msg":"查询成功","data":{"id":"9223372036854775807","orderNo":"R-contract","readerId":"9223372036854775807","sourceType":"preset","productId":null,"diamondAmount":"9007199254740993","priceUsdt":"123456789.12345678","provider":"epusdt","currency":"USDT","token":"USDT","network":"TRC20","gatewayTradeId":"gateway-contract","actualAmount":null,"receiveAddress":"T-contract","paymentUrl":null,"blockTransactionId":null,"status":"pending","gatewayStatus":null,"walletLedgerId":"9223372036854775806","expireTime":"2026-08-15 03:14:05","paidTime":null,"failureCode":null,"failureMessage":null,"createTime":"2026-08-15 03:04:05","updateTime":"2026-08-15 03:04:05"}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, bytes.NewBufferString(test.body))
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			assertContractJSON(t, test.want, response.Body.String())
		})
	}
	if stub.quoted != contractMaxID || stub.created.ReaderID != contractMaxID || stub.created.ProductID == nil || *stub.created.ProductID != contractSafeID || stub.created.RequestID != "request-contract" || stub.getReader != contractMaxID || stub.getOrder != contractMaxID {
		t.Fatalf("recharge calls=%+v", stub)
	}
}

func TestRechargeHTTPErrorContracts(t *testing.T) {
	tests := []struct {
		name, method, path, body, want string
		err                            error
	}{
		{"invalid quote", http.MethodPost, "/reader/recharge/quote", `{"diamondAmount":"not-a-number"}`, `{"code":500,"msg":"充值参数无效","data":null}`, nil},
		{"product unavailable", http.MethodPost, "/reader/me/recharge/orders", `{"productId":"9007199254740993","requestId":"request-error"}`, `{"code":500,"msg":"充值档位不存在或已下架","data":null}`, commercecontract.ErrProductUnavailable},
		{"order not found", http.MethodGet, "/reader/me/recharge/orders/9007199254740993", "", `{"code":500,"msg":"充值订单不存在","data":null}`, commercecontract.ErrNotFound},
		{"invalid order id", http.MethodGet, "/reader/me/recharge/orders/not-a-number", "", `{"code":500,"msg":"internal service error","data":null}`, nil},
		{"repository failure", http.MethodGet, "/reader/recharge/products", "", `{"code":500,"msg":"internal service error","data":null}`, errors.New("database host secret detail")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := contractRouter()
			handler := rechargeHandler{service: &rechargeContract{err: test.err}}
			router.GET("/reader/recharge/products", handler.catalog)
			router.POST("/reader/recharge/quote", handler.quote)
			router.POST("/reader/me/recharge/orders", handler.create)
			router.GET("/reader/me/recharge/orders/:orderId", handler.get)
			request := httptest.NewRequest(test.method, test.path, bytes.NewBufferString(test.body))
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			assertContractJSON(t, test.want, response.Body.String())
		})
	}
}

func TestRegisterRoutesKeepsFrozenReaderCommerceRoutes(t *testing.T) {
	router := gin.New()
	group := router.Group("")
	checkins := &checkinContract{}
	wallets := &walletContract{}
	purchases := &purchaseContract{}
	recharges := &rechargeContract{}
	RegisterCheckinRoutes(group, checkins, nil)
	RegisterWalletRoutes(group, wallets, nil)
	RegisterPurchaseRoutes(group, purchases, nil)
	RegisterRechargeRoutes(group, recharges, nil)

	want := map[string]bool{
		"GET /reader/me/checkin/status":           true,
		"POST /reader/me/checkin":                 true,
		"GET /reader/me/wallet":                   true,
		"GET /reader/me/wallet/ledgers":           true,
		"POST /reader/me/orders/membership":       true,
		"POST /reader/me/orders/chapter":          true,
		"POST /reader/me/orders/book":             true,
		"GET /reader/recharge/products":           true,
		"POST /reader/recharge/quote":             true,
		"POST /reader/me/recharge/orders":         true,
		"GET /reader/me/recharge/orders/:orderId": true,
	}
	got := make(map[string]int)
	for _, route := range router.Routes() {
		got[route.Method+" "+route.Path]++
	}
	if len(got) != len(want) {
		t.Fatalf("route count=%d want=%d routes=%v", len(got), len(want), got)
	}
	for route := range want {
		if got[route] != 1 {
			t.Errorf("route %s count=%d", route, got[route])
		}
	}

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/reader/me/wallet", nil))
	assertContractJSON(t, `{"code":401,"msg":"认证失败，无法访问系统资源","data":null}`, response.Body.String())
}
