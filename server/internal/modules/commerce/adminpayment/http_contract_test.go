package adminpayment

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type contractRepository struct{}

func (contractRepository) List(context.Context, bool) ([]Channel, error) {
	return []Channel{contractChannel()}, nil
}
func (contractRepository) Get(context.Context, int64) (Channel, error) {
	return contractChannel(), nil
}
func (contractRepository) Create(context.Context, ChannelInput) (Channel, error) {
	return contractChannel(), nil
}
func (contractRepository) Update(context.Context, int64, ChannelInput) (Channel, error) {
	return contractChannel(), nil
}
func (contractRepository) Archive(context.Context, int64) error { return nil }
func (contractRepository) Runtime(context.Context) (RuntimeConfig, error) {
	return RuntimeConfig{}, errors.New("not used")
}
func (contractRepository) RuntimeForChannel(context.Context, int64) (RuntimeConfig, error) {
	return RuntimeConfig{}, errors.New("not used")
}

func TestChannelHTTPResponsesNeverExposeCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &Handler{service: NewService(contractRepository{})}
	router := gin.New()
	router.GET("/reader/payment/channels", handler.list)
	router.GET("/reader/payment/channels/:id", handler.get)
	router.POST("/reader/payment/channels", handler.create)
	router.PUT("/reader/payment/channels/:id", handler.update)

	payload := `{
		"displayName":"EPUSDT","provider":"epusdt","enabled":true,
		"currency":"usd","token":"usdt","network":"tron",
		"merchantPid":"merchant-sensitive","secret":"secret-sensitive",
		"epusdtBaseUrl":"https://pay.example","readerBaseUrl":"https://reader.example",
		"createUrl":"https://attacker.example/create","syncUrl":"https://attacker.example/sync",
		"connectTimeoutMs":3000,
		"requestTimeoutMs":10000,"unknownReleaseMinutes":15
	}`
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/reader/payment/channels"},
		{method: http.MethodGet, path: "/reader/payment/channels/9223372036854775001"},
		{method: http.MethodPost, path: "/reader/payment/channels", body: payload},
		{method: http.MethodPut, path: "/reader/payment/channels/9223372036854775001", body: payload},
	}

	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, bytes.NewBufferString(test.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			body := response.Body.String()
			for _, forbidden := range []string{"merchant-sensitive", "secret-sensitive", `"merchantPid"`, `"secret"`, "ciphertext", "attacker.example"} {
				if strings.Contains(body, forbidden) {
					t.Fatalf("response leaked %q: %s", forbidden, body)
				}
			}
			if !strings.Contains(body, `"id":"9223372036854775001"`) || !strings.Contains(body, `"pidConfigured":true`) || !strings.Contains(body, `"secretConfigured":true`) {
				t.Fatalf("response contract incomplete: %s", body)
			}
			for _, required := range []string{`"epusdtBaseUrl":"https://pay.example"`, `"readerBaseUrl":"https://reader.example"`, `"createUrl":"https://pay.example/payments/gmpay/v1/order/create-transaction"`, `"notifyUrl":"https://reader.example/prod-api/reader/payment/epusdt/notify"`, `"syncUrl":"https://pay.example/pay/check-status/{trade_id}"`} {
				if !strings.Contains(body, required) {
					t.Fatalf("response missing derived endpoint %s: %s", required, body)
				}
			}
		})
	}
}

func contractChannel() Channel {
	return Channel{
		ID: 9223372036854775001, DisplayName: "EPUSDT", Provider: "epusdt", Enabled: true,
		Currency: "usd", Token: "usdt", Network: "tron", PIDConfigured: true, SecretConfigured: true,
		EPUSDTBaseURL: "https://pay.example", ReaderBaseURL: "https://reader.example",
		ConnectTimeoutMS: 3000, RequestTimeoutMS: 10000,
		UnknownReleaseMinutes: 15,
	}
}
