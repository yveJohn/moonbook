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
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"github.com/gin-gonic/gin"
)

const shareTextDriverName = "moonbook-reader-invite-share-text"

var (
	registerShareTextDriver sync.Once
	shareTextQueryMode      atomic.Value
)

type shareTextDriver struct{}
type shareTextConn struct{}
type shareTextTx struct{}
type shareTextRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (shareTextDriver) Open(string) (driver.Conn, error)  { return shareTextConn{}, nil }
func (shareTextConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (shareTextConn) Close() error                        { return nil }
func (shareTextConn) Begin() (driver.Tx, error)           { return shareTextTx{}, nil }
func (shareTextConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return shareTextTx{}, nil
}
func (shareTextTx) Commit() error   { return nil }
func (shareTextTx) Rollback() error { return nil }

func (shareTextConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "SELECT code FROM reader_invite_codes"):
		return &shareTextRows{columns: []string{"code"}, values: [][]driver.Value{{"SHARE-CODE"}}}, nil
	case strings.Contains(query, "SELECT count(*) FROM reader_invite_relations"):
		return &shareTextRows{columns: []string{"count"}, values: [][]driver.Value{{int64(0)}}}, nil
	case strings.Contains(query, "FROM sys_params"):
		mode, _ := shareTextQueryMode.Load().(string)
		switch mode {
		case "configured":
			return &shareTextRows{columns: []string{"value"}, values: [][]driver.Value{{"  自定义邀请文案：{{link}}  "}}}, nil
		case "blank":
			return &shareTextRows{columns: []string{"value"}, values: [][]driver.Value{{"   "}}}, nil
		case "missing":
			return &shareTextRows{columns: []string{"value"}}, nil
		default:
			return nil, fmt.Errorf("sys_params unavailable")
		}
	default:
		return nil, fmt.Errorf("unexpected share text query: %s", query)
	}
}

func (r *shareTextRows) Columns() []string { return r.columns }
func (r *shareTextRows) Close() error      { return nil }
func (r *shareTextRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func TestInviteDashboardShareTextUsesSysParams(t *testing.T) {
	registerShareTextDriver.Do(func() {
		sql.Register(shareTextDriverName, shareTextDriver{})
	})
	db, err := sql.Open(shareTextDriverName, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	gin.SetMode(gin.TestMode)
	handler := &Handler{db: db, summary: accountContractSummary{}, rewards: accountContractSummary{}}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("reader.identity", readerauth.Identity{ReaderID: accountSafeID, SessionID: accountMaxID})
		c.Next()
	})
	router.POST("/reader/me/invite/code", handler.inviteDashboard)

	tests := []struct {
		name, mode, want string
	}{
		{"configured template", "configured", "自定义邀请文案：{{link}}"},
		{"blank falls back", "blank", defaultInviteShareText},
		{"missing falls back", "missing", defaultInviteShareText},
		{"query error falls back", "error", defaultInviteShareText},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			shareTextQueryMode.Store(test.mode)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/reader/me/invite/code", nil))
			if resp.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
			}
			var payload struct {
				Code int `json:"code"`
				Data struct {
					ShareTextTemplate string `json:"shareTextTemplate"`
				} `json:"data"`
			}
			if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Code != 200 || payload.Data.ShareTextTemplate != test.want {
				t.Fatalf("shareTextTemplate=%q code=%d body=%s", payload.Data.ShareTextTemplate, payload.Code, resp.Body.String())
			}
		})
	}
}
