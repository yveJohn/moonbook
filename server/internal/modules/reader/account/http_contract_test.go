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
	fixed := time.Date(2026, 8, 15, 5, 6, 7, 0, time.UTC)
	switch {
	case strings.Contains(query, "SELECT target_id FROM commerce_entitlements"):
		return contractRows([]string{"target_id"}, []driver.Value{accountSafeID}), nil
	case strings.Contains(query, "COALESCE(bool_or(permanent)"):
		return contractRows([]string{"permanent", "expires_at"}, []driver.Value{true, nil}), nil
	case strings.Contains(query, "SELECT id,product_type"):
		return contractRows(
			[]string{"id", "product_type", "target_id", "product_name", "price_coin", "allow_bonus_coin", "duration_days", "sale_status", "sort_order", "created_at", "updated_at"},
			[]driver.Value{accountMaxID, "membership", int64(0), "永久会员", accountSafeID, true, int64(30), "on_sale", int64(1), fixed, fixed},
		), nil
	case strings.Contains(query, "SELECT code FROM reader_invite_codes"):
		return contractRows([]string{"code"}, []driver.Value{"ACCOUNT-CONTRACT"}), nil
	case strings.Contains(query, "SELECT invitee_reward_coin"):
		return contractRows([]string{"invitee_reward_coin", "first_recharge"}, []driver.Value{accountSafeID, int64(0)}), nil
	case strings.Contains(query, "SELECT count(*) FROM reader_invite_relations"):
		return contractRows([]string{"count"}, []driver.Value{accountSafeID}), nil
	case strings.Contains(query, "SELECT COALESCE(sum(amount)"):
		return contractRows([]string{"sum"}, []driver.Value{accountMaxID}), nil
	default:
		return nil, fmt.Errorf("unexpected account contract query: %s", query)
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
	handler := &Handler{db: db}
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
			`{"code":200,"msg":"查询成功","data":{"readerId":"9223372036854775807","inviteCode":"ACCOUNT-CONTRACT","inviteCodeAvailable":true,"shareTextTemplate":"邀请你加入月白书城，点击 {{link}} 注册","registerRewardCoin":"9007199254740993","firstRechargeRewardCoin":"0","invitedCount":"9007199254740993","totalRewardCoin":"9223372036854775807","rewards":[]}}`,
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
}
