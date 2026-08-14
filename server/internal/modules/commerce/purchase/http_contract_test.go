package purchase

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"github.com/gin-gonic/gin"
)

const (
	purchaseMaxID  int64 = 9223372036854775807
	purchaseSafeID int64 = 9007199254740993
)

type purchaseContractRepo struct {
	err                error
	membershipReaderID int64
	membershipProduct  string
	membershipRequest  string
	chapterReaderID    int64
	chapterID          string
	chapterPrice       string
	chapterRequest     string
	bookReaderID       int64
	bookID             string
	bookPrice          string
}

func (r *purchaseContractRepo) BuyMembership(_ context.Context, readerID int64, product, request string) (Order, error) {
	if r.err != nil {
		return Order{}, r.err
	}
	r.membershipReaderID, r.membershipProduct, r.membershipRequest = readerID, product, request
	return contractOrder("membership", product, "", ""), nil
}

func (r *purchaseContractRepo) BuyChapter(_ context.Context, readerID int64, chapter, expected, request string) (ChapterResult, error) {
	if r.err != nil {
		return ChapterResult{}, r.err
	}
	r.chapterReaderID, r.chapterID, r.chapterPrice, r.chapterRequest = readerID, chapter, expected, request
	order := contractOrder("chapter", "", chapter, "9007199254740994")
	return ChapterResult{
		PurchaseStatus: "paid",
		Quote:          ChapterQuote{ChapterID: chapter, BookID: "9007199254740994", WordCount: 2001, WordUnit: 1000, CoinUnit: "9007199254740993", PriceCoin: expected},
		Order:          &order,
	}, nil
}

func (r *purchaseContractRepo) BuyBook(_ context.Context, readerID int64, book, expected string) (Order, error) {
	if r.err != nil {
		return Order{}, r.err
	}
	r.bookReaderID, r.bookID, r.bookPrice = readerID, book, expected
	return contractOrder("book", "9007199254740995", book, book), nil
}

func contractOrder(orderType, productID, targetID, bookID string) Order {
	remark := "契约订单"
	paidAt := time.Date(2026, 8, 15, 4, 5, 6, 0, time.UTC)
	return Order{
		ID: "9223372036854775807", ReaderID: "9223372036854775807", OrderNo: "O-contract",
		OrderType: orderType, ProductID: productID, ProductType: orderType, TargetID: targetID,
		BookIDSnapshot: bookID, ProductName: "契约商品", PriceCoin: "9223372036854775807",
		ChapterWordCount: "2001", PricingWordUnit: "1000", PricingCoinUnit: "9007199254740993",
		RechargeCoinAmount: "9007199254740993", BonusCoinAmount: "9223372036854775806",
		Status: "paid", IdempotencyKey: orderType + ":contract", Remark: &remark,
		PaidTime: &paidAt, CreateTime: paidAt, UpdateTime: paidAt,
	}
}

func TestPurchaseHTTPContractKeepsOrderFieldsAndLongValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &purchaseContractRepo{}
	handler := &Handler{service: NewService(repo)}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("reader.identity", readerauth.Identity{ReaderID: purchaseMaxID, SessionID: purchaseSafeID})
		c.Next()
	})
	router.POST("/reader/me/orders/membership", handler.membership)
	router.POST("/reader/me/orders/chapter", handler.chapter)
	router.POST("/reader/me/orders/book", handler.book)

	tests := []struct {
		name, path, body, want string
	}{
		{
			"membership", "/reader/me/orders/membership", `{"productId":"9007199254740993","requestId":"membership-request"}`,
			`{"code":200,"msg":"购买成功","data":{"id":"9223372036854775807","orderNo":"O-contract","readerId":"9223372036854775807","orderType":"membership","productId":"9007199254740993","productType":"membership","targetId":null,"bookIdSnapshot":null,"productNameSnapshot":"契约商品","priceCoinSnapshot":"9223372036854775807","chapterWordCountSnapshot":2001,"pricingWordUnitSnapshot":1000,"pricingCoinUnitSnapshot":"9007199254740993","rechargeCoinAmount":"9007199254740993","bonusCoinAmount":"9223372036854775806","status":"paid","idempotencyKey":"membership:contract","remark":"契约订单","operatorId":null,"paidTime":"2026-08-15 04:05:06","createTime":"2026-08-15 04:05:06","updateTime":"2026-08-15 04:05:06"}}`,
		},
		{
			"chapter", "/reader/me/orders/chapter", `{"chapterId":"9223372036854775807","expectedPrice":"9223372036854775807","requestId":"chapter-request"}`,
			`{"code":200,"msg":"购买成功","data":{"purchaseStatus":"paid","quote":{"chapterId":"9223372036854775807","bookId":"9007199254740994","wordCount":2001,"wordUnit":1000,"coinUnit":"9007199254740993","priceCoin":"9223372036854775807"},"order":{"id":"9223372036854775807","orderNo":"O-contract","readerId":"9223372036854775807","orderType":"chapter","productId":null,"productType":"chapter","targetId":"9223372036854775807","bookIdSnapshot":"9007199254740994","productNameSnapshot":"契约商品","priceCoinSnapshot":"9223372036854775807","chapterWordCountSnapshot":2001,"pricingWordUnitSnapshot":1000,"pricingCoinUnitSnapshot":"9007199254740993","rechargeCoinAmount":"9007199254740993","bonusCoinAmount":"9223372036854775806","status":"paid","idempotencyKey":"chapter:contract","remark":"契约订单","operatorId":null,"paidTime":"2026-08-15 04:05:06","createTime":"2026-08-15 04:05:06","updateTime":"2026-08-15 04:05:06"}}}`,
		},
		{
			"book", "/reader/me/orders/book", `{"bookId":"9007199254740993","expectedPrice":"9223372036854775807"}`,
			`{"code":200,"msg":"购买成功","data":{"id":"9223372036854775807","orderNo":"O-contract","readerId":"9223372036854775807","orderType":"book","productId":"9007199254740995","productType":"book","targetId":"9007199254740993","bookIdSnapshot":"9007199254740993","productNameSnapshot":"契约商品","priceCoinSnapshot":"9223372036854775807","chapterWordCountSnapshot":2001,"pricingWordUnitSnapshot":1000,"pricingCoinUnitSnapshot":"9007199254740993","rechargeCoinAmount":"9007199254740993","bonusCoinAmount":"9223372036854775806","status":"paid","idempotencyKey":"book:contract","remark":"契约订单","operatorId":null,"paidTime":"2026-08-15 04:05:06","createTime":"2026-08-15 04:05:06","updateTime":"2026-08-15 04:05:06"}}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, test.path, bytes.NewBufferString(test.body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
			}
			var want, got any
			if err := json.Unmarshal([]byte(test.want), &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("response mismatch\nwant: %s\n got: %s", test.want, resp.Body.String())
			}
		})
	}
	if repo.membershipReaderID != purchaseMaxID || repo.membershipProduct != "9007199254740993" || repo.membershipRequest != "membership-request" {
		t.Fatalf("membership reader=%d product=%s request=%s", repo.membershipReaderID, repo.membershipProduct, repo.membershipRequest)
	}
	if repo.chapterReaderID != purchaseMaxID || repo.chapterID != "9223372036854775807" || repo.chapterPrice != "9223372036854775807" || repo.chapterRequest != "chapter-request" {
		t.Fatalf("chapter reader=%d id=%s price=%s request=%s", repo.chapterReaderID, repo.chapterID, repo.chapterPrice, repo.chapterRequest)
	}
	if repo.bookReaderID != purchaseMaxID || repo.bookID != "9007199254740993" || repo.bookPrice != "9223372036854775807" {
		t.Fatalf("book reader=%d id=%s price=%s", repo.bookReaderID, repo.bookID, repo.bookPrice)
	}
}

func TestPurchaseHTTPErrorContracts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name, body, want string
		err              error
	}{
		{name: "quote changed", body: `{"bookId":"9007199254740993","expectedPrice":"1"}`, err: ErrQuoteChanged, want: `{"code":46106,"msg":"作品报价已变化，请确认新价格","data":null}`},
		{name: "insufficient balance", body: `{"bookId":"9007199254740993","expectedPrice":"1"}`, err: ErrInsufficientBalance, want: `{"code":500,"msg":"余额不足","data":null}`},
		{name: "product unavailable", body: `{"bookId":"9007199254740993","expectedPrice":"1"}`, err: ErrProductUnavailable, want: `{"code":200,"msg":"购买商品不可用","data":null}`},
		{name: "invalid price", body: `{"bookId":"9007199254740993","expectedPrice":null}`, want: `{"code":200,"msg":"购买参数无效","data":null}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &purchaseContractRepo{err: test.err}
			handler := &Handler{service: NewService(repo)}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("reader.identity", readerauth.Identity{ReaderID: purchaseMaxID, SessionID: purchaseSafeID})
				c.Next()
			})
			router.POST("/reader/me/orders/book", handler.book)
			req := httptest.NewRequest(http.MethodPost, "/reader/me/orders/book", bytes.NewBufferString(test.body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
			}
			var want, got any
			if err := json.Unmarshal([]byte(test.want), &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("response mismatch\nwant: %s\n got: %s", test.want, resp.Body.String())
			}
		})
	}
}
