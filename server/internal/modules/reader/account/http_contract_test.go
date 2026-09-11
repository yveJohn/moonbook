package account

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"github.com/gin-gonic/gin"
)

const (
	accountContractDriverName = "moonbook-reader-account-contract"
	accountMaxID              = int64(9223372036854775807)
	accountSafeID             = int64(9007199254740993)
)

var registerAccountContractDriver sync.Once

type accountContractDriver struct{}
type accountContractConn struct{}
type accountContractTx struct{}
type accountContractRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

type accountContractSummary struct {
	fixed   time.Time
	rewards []commercecontract.InviteRewardRecord
}

func (stub accountContractSummary) Entitlements(_ context.Context, readerID int64) (commercecontract.EntitlementSummary, error) {
	return commercecontract.EntitlementSummary{ReaderID: readerID, BookIDs: []int64{accountSafeID}, MembershipActive: true, MembershipPermanent: true}, nil
}

func (stub accountContractSummary) MembershipProducts(context.Context) ([]commercecontract.MembershipProduct, error) {
	days := 30
	return []commercecontract.MembershipProduct{{ID: accountMaxID, Name: "永久会员", PriceCoin: accountSafeID, AllowBonusCoin: true, DurationDays: &days, SaleStatus: "on_sale", SortOrder: 1, CreatedAt: stub.fixed, UpdatedAt: stub.fixed}}, nil
}

func (stub accountContractSummary) InviteRewardSummary(context.Context, int64) (commercecontract.InviteRewardSummary, error) {
	return commercecontract.InviteRewardSummary{RegisterRewardCoin: accountSafeID, FirstRechargeRewardCoin: 100, TotalRewardCoin: accountMaxID, Records: stub.rewards}, nil
}

func (accountContractDriver) Open(string) (driver.Conn, error)  { return accountContractConn{}, nil }
func (accountContractConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (accountContractConn) Close() error                        { return nil }
func (accountContractConn) Begin() (driver.Tx, error)           { return accountContractTx{}, nil }
func (accountContractConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return accountContractTx{}, nil
}
func (accountContractTx) Commit() error   { return nil }
func (accountContractTx) Rollback() error { return nil }

func (accountContractConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "SELECT code FROM reader_invite_codes"):
		return contractRows([]string{"code"}, []driver.Value{"ACCOUNT-CONTRACT"}), nil
	case strings.Contains(query, "SELECT count(*) FROM reader_invite_relations"):
		return contractRows([]string{"count"}, []driver.Value{accountSafeID}), nil
	case strings.Contains(query, "FROM sys_params"):
		return contractRows([]string{"value"}, []driver.Value{defaultInviteShareText}), nil
	default:
		return nil, fmt.Errorf("unexpected Reader account query: %s", query)
	}
}

func contractRows(columns []string, values ...[]driver.Value) driver.Rows {
	return &accountContractRows{columns: columns, values: values}
}
func (r *accountContractRows) Columns() []string { return r.columns }
func (r *accountContractRows) Close() error      { return nil }
func (r *accountContractRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func TestAccountHTTPContractKeepsEntitlementProductAndInviteLongValues(t *testing.T) {
	registerAccountContractDriver.Do(func() {
		sql.Register(accountContractDriverName, accountContractDriver{})
	})
	db, err := sql.Open(accountContractDriverName, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	gin.SetMode(gin.TestMode)
	fixed := time.Date(2026, 8, 15, 5, 6, 7, 0, time.UTC)
	summary := accountContractSummary{fixed: fixed, rewards: []commercecontract.InviteRewardRecord{
		{ID: accountMaxID, RewardStage: "register", RewardCoin: accountSafeID, GrantedAt: &fixed, Remark: "邀请注册奖励"},
		{ID: accountSafeID, RewardStage: "first_recharge", RewardCoin: 100, Remark: "邀请首充奖励"},
	}}
	handler := &Handler{db: db, summary: summary, rewards: summary}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("reader.identity", readerauth.Identity{ReaderID: accountMaxID, SessionID: accountSafeID})
		c.Next()
	})
	router.GET("/reader/me/entitlements", handler.entitlements)
	router.GET("/reader/products/membership", handler.membershipProducts)
	router.POST("/reader/me/invite/code", handler.inviteDashboard)

	tests := []struct {
		name, method, path, want string
	}{
		{
			"entitlements", http.MethodGet, "/reader/me/entitlements",
			`{"code":200,"msg":"查询成功","data":{"readerId":"9223372036854775807","bookIds":["9007199254740993"],"membershipActive":true,"membershipPermanent":true,"membershipExpireTime":null}}`,
		},
		{
			"membership products", http.MethodGet, "/reader/products/membership",
			`{"code":200,"msg":"查询成功","data":[{"id":"9223372036854775807","productType":"membership","targetId":null,"productName":"永久会员","priceCoin":"9007199254740993","allowBonusCoin":true,"durationDays":30,"saleStatus":"on_sale","sortOrder":1,"remark":"","createTime":"2026-08-15 05:06:07","updateTime":"2026-08-15 05:06:07"}]}`,
		},
		{
			"invite dashboard", http.MethodPost, "/reader/me/invite/code",
			`{"code":200,"msg":"查询成功","data":{"readerId":"9223372036854775807","inviteCode":"ACCOUNT-CONTRACT","inviteCodeAvailable":true,"shareTextTemplate":"邀请你加入月白书城，点击 {{link}} 注册","registerRewardCoin":"9007199254740993","firstRechargeRewardCoin":"100","invitedCount":"9007199254740993","totalRewardCoin":"9223372036854775807","rewards":[{"id":"9223372036854775807","rewardStage":"register","rewardCoin":"9007199254740993","grantTime":"2026-08-15 05:06:07","remark":"邀请注册奖励"},{"id":"9007199254740993","rewardStage":"first_recharge","rewardCoin":"100","grantTime":null,"remark":"邀请首充奖励"}]}}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, httptest.NewRequest(test.method, test.path, nil))
			if resp.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
			}
			var want, got any
			if err := json.Unmarshal([]byte(test.want), &want); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode response: %v; body=%s", err, resp.Body.String())
			}
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("response mismatch\nwant: %s\n got: %s", test.want, resp.Body.String())
			}
		})
	}

	handler.rewards = accountContractSummary{}
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/reader/me/invite/code", nil))
	var payload struct {
		Data struct {
			Rewards []any `json:"rewards"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Rewards == nil || len(payload.Data.Rewards) != 0 {
		t.Fatalf("empty rewards must be []: %s", resp.Body.String())
	}
}
