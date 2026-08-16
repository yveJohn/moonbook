package provider

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/catalog"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/checkin"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/epusdt"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/purchase"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/recharge"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/wallet"
	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
	readercontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/contract"
)

type checkinRepositoryStub struct{ err error }

func (stub checkinRepositoryStub) Status(context.Context, int64) (checkin.Status, error) {
	return checkin.Status{TodayRewardCoin: 9, CheckinAvailable: true}, stub.err
}
func (stub checkinRepositoryStub) Checkin(context.Context, int64) (checkin.Status, error) {
	return checkin.Status{TodayChecked: true}, stub.err
}

func TestCheckinProviderConvertsStatusAndClassifiesErrors(t *testing.T) {
	provider := NewCheckin(checkin.NewService(checkinRepositoryStub{}))
	status, err := provider.CheckinStatus(context.Background(), 1)
	if err != nil || status.TodayRewardCoin != 9 || !status.CheckinAvailable {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	provider = NewCheckin(checkin.NewService(checkinRepositoryStub{err: checkin.ErrUnavailable}))
	if _, err := provider.Checkin(context.Background(), 1); !errors.Is(err, contract.ErrProductUnavailable) {
		t.Fatalf("error=%v", err)
	}
}

type walletRepositoryStub struct{}

func (walletRepositoryStub) Get(context.Context, int64) (wallet.Wallet, error) {
	return wallet.Wallet{ReaderID: 7, RechargeCoinBalance: 8}, nil
}
func (walletRepositoryStub) List(context.Context, int64, string, int, int) ([]wallet.Ledger, int64, error) {
	return []wallet.Ledger{{ID: 9, ReaderID: 7, Amount: 3, CreatedAt: time.Unix(1, 0)}}, 1, nil
}
func (walletRepositoryStub) Mutate(context.Context, wallet.Mutation) (wallet.Ledger, error) {
	return wallet.Ledger{}, nil
}

func TestWalletProviderConvertsBalancesAndLedgers(t *testing.T) {
	provider := NewWallet(wallet.NewService(walletRepositoryStub{}))
	balance, err := provider.Wallet(context.Background(), 7)
	if err != nil || balance.ReaderID != 7 || balance.RechargeCoinBalance != 8 {
		t.Fatalf("wallet=%+v err=%v", balance, err)
	}
	rows, total, err := provider.WalletLedgers(context.Background(), 7, "recharge", 1, 20)
	if err != nil || total != 1 || len(rows) != 1 || rows[0].ID != 9 || rows[0].Amount != 3 {
		t.Fatalf("rows=%+v total=%d err=%v", rows, total, err)
	}
}

type purchaseRepositoryStub struct {
	err                           error
	product, chapter, price, book string
}

func (stub *purchaseRepositoryStub) LockIdempotency(context.Context, int64) error { return nil }
func (stub *purchaseRepositoryStub) FindOrder(context.Context, int64, string) (purchase.Order, error) {
	return purchase.Order{}, sql.ErrNoRows
}
func (stub *purchaseRepositoryStub) BuyMembership(_ context.Context, _ int64, product int64, _ string) (purchase.Order, error) {
	stub.product = strconv.FormatInt(product, 10)
	return purchase.Order{ID: "11", ReaderID: "7", ProductID: stub.product, PriceCoin: "9"}, stub.err
}
func (stub *purchaseRepositoryStub) BuyChapter(_ context.Context, _ int64, snapshot novelcontract.PurchaseSnapshot, price int64, _ string) (purchase.ChapterResult, error) {
	stub.chapter, stub.price = strconv.FormatInt(snapshot.TargetID, 10), strconv.FormatInt(price, 10)
	order := purchase.Order{ID: "12", ReaderID: "7", TargetID: stub.chapter, PriceCoin: stub.price}
	return purchase.ChapterResult{PurchaseStatus: "paid", Quote: purchase.ChapterQuote{ChapterID: stub.chapter, BookID: "5", CoinUnit: "2", PriceCoin: stub.price}, Order: &order}, stub.err
}
func (stub *purchaseRepositoryStub) BuyBook(_ context.Context, _ int64, snapshot novelcontract.PurchaseSnapshot, price int64, _ string) (purchase.Order, error) {
	stub.book, stub.price = strconv.FormatInt(snapshot.TargetID, 10), strconv.FormatInt(price, 10)
	return purchase.Order{}, stub.err
}

type purchaseTransactor struct{}

func (purchaseTransactor) Within(ctx context.Context, callback func(context.Context) error) error {
	return callback(ctx)
}

type purchaseReader struct{}

func (purchaseReader) LockAccount(_ context.Context, readerID int64) (readercontract.Account, error) {
	return readercontract.Account{ID: readerID, Status: "enabled"}, nil
}

type purchaseNovel struct{}

func (purchaseNovel) LockPurchaseSnapshot(_ context.Context, targetType string, targetID int64) (novelcontract.PurchaseSnapshot, error) {
	bookID := targetID
	if targetType == "chapter" {
		bookID = 5
	}
	return novelcontract.PurchaseSnapshot{Type: targetType, TargetID: targetID, BookID: bookID, Enabled: true}, nil
}

func TestPurchaseProviderConvertsLongValuesAndErrors(t *testing.T) {
	repository := &purchaseRepositoryStub{}
	provider := NewPurchase(purchase.NewService(repository, purchaseTransactor{}, purchaseReader{}, purchaseNovel{}))
	order, err := provider.BuyMembership(context.Background(), 7, 9007199254740993, "request")
	if err != nil || repository.product != "9007199254740993" || order.ID != 11 || order.ProductID == nil || *order.ProductID != 9007199254740993 {
		t.Fatalf("order=%+v repository=%+v err=%v", order, repository, err)
	}
	result, err := provider.BuyChapter(context.Background(), 7, 9223372036854775807, 9007199254740993, "request")
	if err != nil || repository.chapter != "9223372036854775807" || repository.price != "9007199254740993" || result.Order == nil || result.Quote.ChapterID != 9223372036854775807 {
		t.Fatalf("result=%+v repository=%+v err=%v", result, repository, err)
	}
	repository.err = purchase.ErrInsufficientBalance
	if _, err := provider.BuyBook(context.Background(), 7, 5, 9); !errors.Is(err, contract.ErrInsufficientFunds) {
		t.Fatalf("error=%v", err)
	}
}

type rechargeRepositoryStub struct {
	err     error
	request recharge.CreateRequest
	orderID string
}

func (stub *rechargeRepositoryStub) Catalog(context.Context) (recharge.Catalog, error) {
	return recharge.Catalog{Products: []recharge.Product{{ID: 4, ProductName: "P", DiamondAmount: 5}}}, stub.err
}
func (stub *rechargeRepositoryStub) Quote(context.Context, int64) (recharge.Quote, error) {
	return recharge.Quote{DiamondAmount: "6", PriceUSDT: "1.2"}, stub.err
}
func (stub *rechargeRepositoryStub) Prepare(_ context.Context, request recharge.CreateRequest) (recharge.PreparedOrder, error) {
	stub.request = request
	return recharge.PreparedOrder{ReaderID: request.ReaderID, RequestID: request.RequestID, ProductID: request.ProductID}, stub.err
}
func (stub *rechargeRepositoryStub) Start(context.Context, recharge.PreparedOrder) (recharge.StartResult, error) {
	return recharge.StartResult{Order: recharge.Order{ID: "8", ReaderID: "7", DiamondAmount: "6", ProductID: "4", WalletLedgerID: stringPointer("9")}}, stub.err
}
func (stub *rechargeRepositoryStub) Complete(context.Context, string, epusdt.CreateResponse) (recharge.Order, error) {
	return recharge.Order{}, stub.err
}
func (stub *rechargeRepositoryStub) Fail(context.Context, string, *epusdt.GatewayError) (recharge.Order, error) {
	return recharge.Order{}, stub.err
}
func (stub *rechargeRepositoryStub) GetOrder(_ context.Context, _ int64, orderID string) (recharge.Order, error) {
	stub.orderID = orderID
	return recharge.Order{}, stub.err
}

func TestRechargeProviderConvertsRequestsOrdersAndErrors(t *testing.T) {
	repository := &rechargeRepositoryStub{}
	provider := NewRecharge(recharge.NewService(repository, providerRechargeGateway{}, recharge.GatewaySnapshot{CredentialRef: "primary", MerchantPID: "merchant"}, nil))
	productID, amount := int64(4), int64(6)
	order, err := provider.CreateRechargeOrder(context.Background(), contract.RechargeCreateRequest{ReaderID: 7, ProductID: &productID, CustomDiamondAmount: &amount, RequestID: "request"})
	if err != nil || repository.request.CustomDiamondAmount != "6" || order.ID != 8 || order.ProductID == nil || *order.ProductID != 4 || order.WalletLedgerID == nil || *order.WalletLedgerID != 9 {
		t.Fatalf("order=%+v request=%+v err=%v", order, repository.request, err)
	}
	if _, err := provider.RechargeOrder(context.Background(), 7, 9223372036854775807); err != nil || repository.orderID != "9223372036854775807" {
		t.Fatalf("orderID=%s err=%v", repository.orderID, err)
	}
	repository.err = sql.ErrNoRows
	if _, err := provider.RechargeOrder(context.Background(), 7, 1); !errors.Is(err, contract.ErrNotFound) {
		t.Fatalf("error=%v", err)
	}
}

type providerRechargeGateway struct{}

func (providerRechargeGateway) Create(context.Context, epusdt.CreateRequest) (epusdt.CreateResponse, error) {
	return epusdt.CreateResponse{}, nil
}

func stringPointer(value string) *string { return &value }

type accessRepositoryStub struct{ context catalog.AccessContext }

func (stub accessRepositoryStub) LoadAccessContext(context.Context, catalog.AccessRequest) (catalog.AccessContext, error) {
	return stub.context, nil
}

func TestAccessProviderConvertsCatalogDecision(t *testing.T) {
	product := catalog.Product{ID: 9007199254740993, ProductName: "作品", PriceCoin: 11, SaleStatus: "on_sale"}
	provider := NewAccess(catalog.NewService(accessRepositoryStub{context: catalog.AccessContext{Product: &product, Reader: catalog.ReaderContext{ReaderID: int64Pointer(7)}}}))
	results, err := provider.AccessReaders(context.Background(), []contract.AccessRequest{{ReaderID: int64Pointer(7), BookID: 9223372036854775807, ChargeMode: "fixed_price"}})
	if err != nil || len(results) != 1 || results[0].BookID != 9223372036854775807 || results[0].ProductID == nil || *results[0].ProductID != 9007199254740993 || results[0].PriceCoin != 11 {
		t.Fatalf("results=%+v err=%v", results, err)
	}
}

func int64Pointer(value int64) *int64 { return &value }
