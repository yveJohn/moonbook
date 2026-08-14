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
			Reader      struct {
				ReaderID string `json:"readerId"`
			} `json:"reader"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != 200 || body.Msg == "" || body.Data.AccessToken == "" || body.Data.Reader.ReaderID != "9223372036854775807" {
		t.Fatalf("body=%s", resp.Body.String())
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
