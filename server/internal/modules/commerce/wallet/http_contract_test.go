package wallet

import (
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
	walletMaxID  int64 = 9223372036854775807
	walletSafeID int64 = 9007199254740993
)

type walletContractRepo struct {
	coinType   string
	page, size int
}

func (r *walletContractRepo) Get(context.Context, int64) (Wallet, error) {
	return Wallet{
		ReaderID: walletMaxID, RechargeCoinBalance: walletMaxID, BonusCoinBalance: walletSafeID,
		TotalRechargeCoinIncome: walletMaxID - 1, TotalBonusCoinIncome: walletSafeID + 1,
		TotalRechargeCoinExpense: walletMaxID - 2, TotalBonusCoinExpense: walletSafeID + 2,
	}, nil
}

func (r *walletContractRepo) List(_ context.Context, readerID int64, coinType string, page, size int) ([]Ledger, int64, error) {
	r.coinType, r.page, r.size = coinType, page, size
	bizID, remark := "9223372036854775806", "契约流水"
	return []Ledger{{
		ID: walletMaxID, ReaderID: readerID, Amount: walletSafeID, BalanceBefore: walletMaxID - 1,
		BalanceAfter: walletMaxID, LedgerNo: "L-contract", BizType: "recharge", Direction: "income",
		CoinType: coinType, BizID: &bizID, Remark: &remark,
		CreatedAt: time.Date(2026, 8, 15, 2, 3, 4, 0, time.UTC),
	}}, 1, nil
}

func (r *walletContractRepo) Mutate(context.Context, Mutation) (Ledger, error) {
	return Ledger{}, nil
}

func TestWalletHTTPContractKeepsIDsAndAmountsAsStrings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &walletContractRepo{}
	handler := &Handler{service: NewService(repo)}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("reader.identity", readerauth.Identity{ReaderID: walletMaxID, SessionID: walletSafeID})
		c.Next()
	})
	router.GET("/reader/me/wallet", handler.get)
	router.GET("/reader/me/wallet/ledgers", handler.ledgers)

	tests := []struct {
		name, path, want string
	}{
		{
			"wallet", "/reader/me/wallet",
			`{"code":200,"msg":"查询成功","data":{"readerId":"9223372036854775807","rechargeCoinBalance":"9223372036854775807","bonusCoinBalance":"9007199254740993","totalRechargeCoinIncome":"9223372036854775806","totalBonusCoinIncome":"9007199254740994","totalRechargeCoinExpense":"9223372036854775805","totalBonusCoinExpense":"9007199254740995","expiringBonusCoin":"0"}}`,
		},
		{
			"ledger page", "/reader/me/wallet/ledgers?coinType=bonus&pageNum=0&pageSize=1000",
			`{"code":200,"msg":"查询成功","rows":[{"id":"9223372036854775807","readerId":"9223372036854775807","ledgerNo":"L-contract","bizType":"recharge","bizId":"9223372036854775806","orderNo":null,"direction":"income","coinType":"bonus","amount":"9007199254740993","balanceBefore":"9223372036854775806","balanceAfter":"9223372036854775807","remark":"契约流水","createTime":"2026-08-15T02:03:04Z"}],"total":1}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, test.path, nil))
			if resp.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
			}
			assertWalletJSON(t, test.want, resp.Body.String())
		})
	}
	if repo.coinType != "bonus" || repo.page != 1 || repo.size != 100 {
		t.Fatalf("ledger query coin=%s page=%d size=%d", repo.coinType, repo.page, repo.size)
	}
}

func assertWalletJSON(t *testing.T, want, got string) {
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
