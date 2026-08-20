//go:build integration

package adminbootstrap

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

const managementTestConfirmation = "moonbook_management"

type managementEnvelope struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
	Msg  string          `json:"msg"`
}

type managementClient struct {
	baseURL string
	http    *http.Client
}

type testAdmin struct {
	ID       int64
	Username string
	Password string
}

func TestManagementFoundationHTTP(t *testing.T) {
	if os.Getenv("MOONBOOK_MANAGEMENT_TEST_CONFIRM") != managementTestConfirmation {
		t.Skip("MOONBOOK_MANAGEMENT_TEST_CONFIRM 未配置")
	}
	baseURL := strings.TrimRight(os.Getenv("MOONBOOK_MANAGEMENT_TEST_BASE_URL"), "/")
	if err := validateLoopbackBaseURL(baseURL); err != nil {
		t.Fatal(err)
	}
	db, integrationConfig := integrationtest.RequireDB(t)
	redisClient := integrationtest.RequireRedis(t, integrationConfig)
	client := managementClient{baseURL: baseURL, http: &http.Client{Timeout: 10 * time.Second}}
	prefix := strings.ReplaceAll(integrationtest.Prefix(), "-", "_")
	full := seedTestAdmin(t, db, prefix+"_full", 888, true)
	limited := seedTestAdmin(t, db, prefix+"_limited", 9528, false)
	var issuedTokens []string
	t.Cleanup(func() { cleanupTestAdmins(t, db, []testAdmin{full, limited}, issuedTokens) })

	status, forcedLogin, raw := client.login(t, redisClient, full)
	requireManagementCode(t, status, forcedLogin, raw, 0)
	forcedToken, forcedUserID, mustChange := loginData(t, forcedLogin.Data)
	issuedTokens = append(issuedTokens, forcedToken)
	if !mustChange || forcedUserID != full.ID {
		t.Fatalf("forced login user=%d needChangePassword=%v", forcedUserID, mustChange)
	}

	status, _, _ = client.request(t, http.MethodPost, "user/getUserList", forcedToken, full.ID, map[string]any{"page": 1, "pageSize": 10})
	if status != http.StatusConflict {
		t.Fatalf("must-change token list status=%d want=%d", status, http.StatusConflict)
	}
	newPassword := full.Password + "-Changed!"
	status, changed, raw := client.request(t, http.MethodPost, "user/changePassword", forcedToken, full.ID, map[string]any{"password": full.Password, "newPassword": newPassword})
	requireManagementCode(t, status, changed, raw, 0)
	full.Password = newPassword

	status, fullLogin, raw := client.login(t, redisClient, full)
	requireManagementCode(t, status, fullLogin, raw, 0)
	fullToken, fullUserID, mustChange := loginData(t, fullLogin.Data)
	issuedTokens = append(issuedTokens, fullToken)
	if mustChange || fullUserID != full.ID {
		t.Fatalf("changed login user=%d needChangePassword=%v", fullUserID, mustChange)
	}

	readChecks := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodPost, "authority/getAuthorityList", map[string]any{"page": 1, "pageSize": 10}},
		{http.MethodPost, "casbin/getPolicyPathByAuthorityId", map[string]any{"authorityId": 888}},
		{http.MethodPost, "department/getDepartmentList", map[string]any{}},
		{http.MethodPost, "position/getPositionList", map[string]any{"page": 1, "pageSize": 10}},
		{http.MethodGet, "sysDictionary/getSysDictionaryList?page=1&pageSize=10", nil},
		{http.MethodGet, "sysParams/getSysParamsList?page=1&pageSize=10", nil},
		{http.MethodGet, "sysLoginLog/getLoginLogList?page=1&pageSize=10", nil},
		{http.MethodGet, "sysOperationRecord/getSysOperationRecordList?page=1&pageSize=10", nil},
		{http.MethodGet, "timedTask/getTimedTaskList?page=1&pageSize=10", nil},
		{http.MethodPost, "dataAccessLog/getDataAccessLogList", map[string]any{"page": 1, "pageSize": 10}},
	}
	for _, check := range readChecks {
		status, envelope, raw := client.request(t, check.method, check.path, fullToken, full.ID, check.body)
		requireManagementCode(t, status, envelope, raw, 0)
	}

	status, configResponse, raw := client.request(t, http.MethodPost, "system/getSystemConfig", fullToken, full.ID, map[string]any{})
	requireManagementCode(t, status, configResponse, raw, 0)
	assertNoSensitiveConfigKeys(t, configResponse.Data)
	status, writeResponse, _ := client.request(t, http.MethodPost, "system/setSystemConfig", fullToken, full.ID, map[string]any{})
	if status != http.StatusNotFound && writeResponse.Code == 0 {
		t.Fatalf("system configuration write unexpectedly succeeded: status=%d response=%+v", status, writeResponse)
	}

	status, limitedLogin, raw := client.login(t, redisClient, limited)
	requireManagementCode(t, status, limitedLogin, raw, 0)
	limitedToken, limitedUserID, _ := loginData(t, limitedLogin.Data)
	issuedTokens = append(issuedTokens, limitedToken)
	status, denied, raw := client.request(t, http.MethodPost, "department/getDepartmentList", limitedToken, limitedUserID, map[string]any{})
	if status != http.StatusOK || denied.Code == 0 || !strings.Contains(denied.Msg, "权限") {
		t.Fatalf("limited role was not denied: status=%d body=%s", status, raw)
	}

	status, logout, raw := client.request(t, http.MethodPost, "jwt/jsonInBlacklist", fullToken, full.ID, nil)
	requireManagementCode(t, status, logout, raw, 0)
	status, revoked, raw := client.request(t, http.MethodPost, "user/getUserList", fullToken, full.ID, map[string]any{"page": 1, "pageSize": 10})
	if status == http.StatusOK && revoked.Code == 0 {
		t.Fatalf("revoked token remained usable: status=%d body=%s", status, raw)
	}
}

func (c managementClient) login(t *testing.T, redisClient *redis.Client, admin testAdmin) (int, managementEnvelope, string) {
	t.Helper()
	status, captcha, raw := c.request(t, http.MethodPost, "base/captcha", "", 0, nil)
	requireManagementCode(t, status, captcha, raw, 0)
	var captchaData struct {
		CaptchaID string `json:"captchaId"`
	}
	if err := json.Unmarshal(captcha.Data, &captchaData); err != nil || captchaData.CaptchaID == "" {
		t.Fatalf("invalid captcha data: err=%v data=%s", err, captcha.Data)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	answer, err := redisClient.Get(ctx, "CAPTCHA_"+captchaData.CaptchaID).Result()
	if err != nil || answer == "" {
		t.Fatalf("read isolated captcha answer: %v", err)
	}
	return c.request(t, http.MethodPost, "base/login", "", 0, map[string]any{
		"username": admin.Username, "password": admin.Password, "captcha": answer, "captchaId": captchaData.CaptchaID,
	})
}

func validateLoopbackBaseURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return fmt.Errorf("MOONBOOK_MANAGEMENT_TEST_BASE_URL must be an HTTP(S) URL")
	}
	if !strings.EqualFold(parsed.Hostname(), "localhost") {
		ip := net.ParseIP(parsed.Hostname())
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("MOONBOOK_MANAGEMENT_TEST_BASE_URL must use a loopback host")
		}
	}
	return nil
}

func seedTestAdmin(t *testing.T, db *sql.DB, username string, authorityID int64, mustChange bool) testAdmin {
	t.Helper()
	password := "Management-Test-2026!"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	err = db.QueryRow(`INSERT INTO sys_users
        (created_at,updated_at,uuid,username,password,nick_name,authority_id,enable,password_updated_at,must_change_password)
        VALUES(now(),now(),$1,$2,$3,'Management HTTP Fixture',$4,1,now(),$5) RETURNING id`,
		uuid.NewString(), username, string(hash), authorityID, mustChange).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO sys_user_authority(sys_user_id,sys_authority_authority_id) VALUES($1,$2)`, id, authorityID); err != nil {
		t.Fatal(err)
	}
	return testAdmin{ID: id, Username: username, Password: password}
}

func cleanupTestAdmins(t *testing.T, db *sql.DB, admins []testAdmin, tokens []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for _, token := range tokens {
		_, _ = db.ExecContext(ctx, `DELETE FROM jwt_blacklists WHERE jwt=$1`, token)
	}
	for _, admin := range admins {
		_, _ = db.ExecContext(ctx, `DELETE FROM sys_data_access_logs WHERE user_id=$1`, admin.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM sys_operation_records WHERE user_id=$1`, admin.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM sys_login_logs WHERE username=$1 OR user_id=$2`, admin.Username, admin.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM sys_user_authority WHERE sys_user_id=$1`, admin.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM sys_users WHERE id=$1`, admin.ID)
	}
}

func (c managementClient) request(t *testing.T, method, path, token string, userID int64, body any) (int, managementEnvelope, string) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, c.baseURL+"/"+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("x-token", token)
		request.Header.Set("x-user-id", fmt.Sprintf("%d", userID))
	}
	response, err := c.http.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	rawBytes, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		t.Fatal(err)
	}
	raw := string(rawBytes)
	var envelope managementEnvelope
	_ = json.Unmarshal(rawBytes, &envelope)
	return response.StatusCode, envelope, raw
}

func requireManagementCode(t *testing.T, status int, envelope managementEnvelope, raw string, want int) {
	t.Helper()
	if status != http.StatusOK || envelope.Code != want {
		t.Fatalf("status=%d code=%d want=%d body=%s", status, envelope.Code, want, raw)
	}
}

func loginData(t *testing.T, raw json.RawMessage) (string, int64, bool) {
	t.Helper()
	var data struct {
		Token              string `json:"token"`
		NeedChangePassword bool   `json:"needChangePassword"`
		User               struct {
			ID int64 `json:"ID"`
		} `json:"user"`
	}
	if err := json.Unmarshal(raw, &data); err != nil || data.Token == "" || data.User.ID == 0 {
		t.Fatalf("invalid login data: err=%v data=%s", err, raw)
	}
	return data.Token, data.User.ID, data.NeedChangePassword
}

func assertNoSensitiveConfigKeys(t *testing.T, raw json.RawMessage) {
	t.Helper()
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	forbidden := map[string]bool{
		"password": true, "secret": true, "signing-key": true, "access-key": true,
		"access-key-id": true, "access-key-secret": true, "secret-key": true, "token": true,
	}
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				if forbidden[strings.ToLower(key)] {
					t.Fatalf("configuration response exposed sensitive key %q", key)
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(data)
}
