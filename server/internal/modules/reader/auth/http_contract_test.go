package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type contractRegistration struct{}

func (contractRegistration) Register(context.Context, RegisterRequest) (ReaderAccount, AccessToken, error) {
	return ReaderAccount{ID: 9223372036854775807, Username: "reader", Nickname: "读者", Status: AccountStatusEnabled}, AccessToken{
		AccessToken: "reader-token", ExpiresAt: time.Now().Add(time.Hour),
	}, nil
}

func TestLoginHTTPContractKeepsLongReaderIDAsString(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hashBytes, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	hash := string(hashBytes)
	repo := &memoryRepository{account: ReaderAccount{
		ID: 9223372036854775807, Username: "reader", PasswordHash: hash,
		PasswordAlgorithm: PasswordAlgorithmBcrypt, Status: AccountStatusEnabled,
	}}
	router := gin.New()
	RegisterRoutes(router.Group("/"), NewHandler(newServiceForTest(repo, &fixedLimiter{allowed: true}), contractRegistration{}))
	req := httptest.NewRequest(http.MethodPost, "/reader/auth/login", bytes.NewBufferString(`{"username":"reader","password":"correct-password"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	var body struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			AccessToken string `json:"accessToken"`
			ExpireIn    int64  `json:"expireIn"`
			Reader      struct {
				ReaderID string `json:"readerId"`
				Username string `json:"username"`
				Nickname string `json:"nickname"`
				Status   string `json:"status"`
			} `json:"reader"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != 200 || body.Msg != "操作成功" || body.Data.AccessToken == "" || body.Data.ExpireIn < 3598 || body.Data.ExpireIn > 3600 || body.Data.Reader.ReaderID != "9223372036854775807" || body.Data.Reader.Username != "reader" || body.Data.Reader.Nickname != "" || body.Data.Reader.Status != AccountStatusEnabled {
		t.Fatalf("body=%s", resp.Body.String())
	}
}

func TestRegisterHTTPContractReturnsFrozenLoginShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router.Group("/"), NewHandler(newServiceForTest(&memoryRepository{}, &fixedLimiter{allowed: true}), contractRegistration{}))
	req := httptest.NewRequest(http.MethodPost, "/reader/auth/register", bytes.NewBufferString(`{"username":"reader","password":"correct-password","nickname":"读者","inviteCode":"MBTESTCODE"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	data, ok := body["data"].(map[string]any)
	reader, readerOK := data["reader"].(map[string]any)
	if resp.Code != http.StatusOK || body["code"] != float64(200) || body["msg"] != "操作成功" || !ok || data["accessToken"] != "reader-token" || !readerOK || reader["readerId"] != "9223372036854775807" || reader["username"] != "reader" || reader["nickname"] != "读者" || reader["status"] != AccountStatusEnabled {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	expireIn, ok := data["expireIn"].(float64)
	if !ok || expireIn < 3598 || expireIn > 3600 {
		t.Fatalf("expireIn=%v body=%s", data["expireIn"], resp.Body.String())
	}
}

func TestProfilePasswordAndLogoutHTTPContracts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hashBytes, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	repo := &memoryRepository{account: ReaderAccount{
		ID: 9007199254740993, Username: "reader", Nickname: "测试读者", PasswordHash: string(hashBytes),
		PasswordAlgorithm: PasswordAlgorithmBcrypt, Status: AccountStatusEnabled,
	}}
	service := newServiceForTest(repo, &fixedLimiter{allowed: true})
	token, err := service.Login(context.Background(), "reader", "old-password", "203.0.113.8")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRoutes(router.Group("/"), NewHandler(service, contractRegistration{}))
	request := func(method, path, payload string) map[string]any {
		t.Helper()
		var body *bytes.Buffer
		if payload == "" {
			body = bytes.NewBuffer(nil)
		} else {
			body = bytes.NewBufferString(payload)
		}
		req := httptest.NewRequest(method, path, body)
		req.Header.Set("Authorization", "Bearer "+token.AccessToken)
		if payload != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("%s %s status=%d body=%s", method, path, resp.Code, resp.Body.String())
		}
		var decoded map[string]any
		if err := json.Unmarshal(resp.Body.Bytes(), &decoded); err != nil {
			t.Fatal(err)
		}
		return decoded
	}
	profile := request(http.MethodGet, "/reader/auth/profile", "")
	profileData, ok := profile["data"].(map[string]any)
	if profile["code"] != float64(200) || profile["msg"] != "查询成功" || !ok || profileData["readerId"] != "9007199254740993" || profileData["username"] != "reader" || profileData["nickname"] != "测试读者" || profileData["status"] != AccountStatusEnabled {
		t.Fatalf("profile=%v", profile)
	}
	password := request(http.MethodPut, "/reader/auth/password", `{"currentPassword":"old-password","newPassword":"new-password","confirmPassword":"new-password"}`)
	if password["code"] != float64(200) || password["msg"] != "操作成功" || password["data"] != nil {
		t.Fatalf("password=%v", password)
	}
	if bcrypt.CompareHashAndPassword([]byte(repo.account.PasswordHash), []byte("new-password")) != nil {
		t.Fatal("password was not replaced")
	}
	logout := request(http.MethodPost, "/reader/auth/logout", "")
	if logout["code"] != float64(200) || logout["msg"] != "操作成功" || logout["data"] != nil {
		t.Fatalf("logout=%v", logout)
	}
	if _, err := service.ValidateToken(context.Background(), token.AccessToken); err == nil {
		t.Fatal("logout left the session valid")
	}
}

func TestProtectedHTTPContractUsesReaderCompatibilityError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Next() })
	RegisterRoutes(router.Group("/"), NewHandler(newServiceForTest(&memoryRepository{}, &fixedLimiter{allowed: true}), contractRegistration{}))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/reader/auth/profile", nil))
	var body struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if resp.Code != http.StatusOK || body.Code != 401 || body.Msg == "" {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestProtectedHTTPContractRejectsExpiredRevokedAndDisabledSessions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name   string
		mutate func(*memoryRepository)
	}{
		{
			name: "expired",
			mutate: func(repo *memoryRepository) {
				session := repo.sessions[1]
				session.ExpiresAt = time.Now().Add(-time.Minute)
				repo.sessions[1] = session
			},
		},
		{
			name: "revoked",
			mutate: func(repo *memoryRepository) {
				session := repo.sessions[1]
				revokedAt := time.Now()
				session.RevokedAt = &revokedAt
				repo.sessions[1] = session
			},
		},
		{
			name: "disabled account",
			mutate: func(repo *memoryRepository) {
				repo.account.Status = AccountStatusDisabled
			},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			hashBytes, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
			if err != nil {
				t.Fatal(err)
			}
			repo := &memoryRepository{account: ReaderAccount{
				ID: 9007199254740993, Username: "reader", PasswordHash: string(hashBytes),
				PasswordAlgorithm: PasswordAlgorithmBcrypt, Status: AccountStatusEnabled,
			}}
			service := newServiceForTest(repo, &fixedLimiter{allowed: true})
			token, err := service.Login(context.Background(), "reader", "correct-password", "203.0.113.8")
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(repo)

			router := gin.New()
			RegisterRoutes(router.Group("/"), NewHandler(service, contractRegistration{}))
			req := httptest.NewRequest(http.MethodGet, "/reader/auth/profile", nil)
			req.Header.Set("Authorization", "Bearer "+token.AccessToken)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			var body struct {
				Code int    `json:"code"`
				Msg  string `json:"msg"`
			}
			if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if resp.Code != http.StatusOK || body.Code != 401 || body.Msg != "认证失败，无法访问系统资源" {
				t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
			}
		})
	}
}
