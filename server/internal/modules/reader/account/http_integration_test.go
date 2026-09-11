//go:build integration

package account

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"github.com/gin-gonic/gin"
)

func TestInviteDashboardReadsShareTextFromSysParams(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('public.sys_params') IS NOT NULL`).Scan(&exists); err != nil || !exists {
		t.Fatal("sys_params 不存在，请先执行项目迁移")
	}
	readerID := time.Now().UnixNano()
	username := fmt.Sprintf("%s-share", integrationtest.Prefix())
	code := fmt.Sprintf("SHARE-%d", readerID)
	template := fmt.Sprintf("集成邀请文案 {{link}} %d", readerID)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash,status) VALUES($1,$2,'邀请文案',$3,'enabled')`, readerID, username, "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status) VALUES($1,$2,$3,'enabled')`, readerID, code, readerID); err != nil {
		t.Fatal(err)
	}
	var paramID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO sys_params(created_at,updated_at,name,key,value,"desc") VALUES(now(),now(),'读者邀请分享文案',$1,$2,'integration') RETURNING id`, inviteShareTextKey, template).Scan(&paramID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = db.ExecContext(cleanup, `DELETE FROM sys_params WHERE id=$1`, paramID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_invite_codes WHERE id=$1`, readerID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})

	gin.SetMode(gin.TestMode)
	handler := &Handler{db: db, summary: accountContractSummary{}, rewards: accountContractSummary{}}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("reader.identity", readerauth.Identity{ReaderID: readerID, SessionID: readerID})
		c.Next()
	})
	router.POST("/reader/me/invite/code", handler.inviteDashboard)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/reader/me/invite/code", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	var payload struct {
		Code int `json:"code"`
		Data struct {
			InviteCode        string `json:"inviteCode"`
			ShareTextTemplate string `json:"shareTextTemplate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != 200 || payload.Data.InviteCode != code || payload.Data.ShareTextTemplate != template {
		t.Fatalf("unexpected invite dashboard: %s", resp.Body.String())
	}
}
