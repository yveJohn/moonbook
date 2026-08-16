package payment

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/epusdt"
	"github.com/gin-gonic/gin"
)

func TestCallbackHTTPUsesOrderCredentialAndPreservesGatewayResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	credentials, err := epusdt.NewCredentialProvider("primary", "merchant-current", "current-secret", `[{"ref":"previous","pid":"merchant-previous","secret":"previous-secret"}]`)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{
		"pid": "merchant-previous", "trade_id": "trade", "order_id": "RC1", "amount": "2.00", "actual_amount": "2.00",
		"receive_address": "T-address", "token": "usdt", "block_transaction_id": "tx", "status": "2",
	}
	fields["signature"] = Sign(fields, "previous-secret")

	for _, test := range []struct {
		name       string
		signature  string
		wantStatus int
		wantBody   string
		wantCalls  int
	}{
		{name: "valid historical credential", signature: fields["signature"], wantStatus: http.StatusOK, wantBody: "success", wantCalls: 1},
		{name: "invalid signature", signature: Sign(fields, "wrong-secret"), wantStatus: http.StatusUnauthorized, wantBody: "fail", wantCalls: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			payload := make(map[string]string, len(fields))
			for key, value := range fields {
				payload[key] = value
			}
			payload["signature"] = test.signature
			body, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			repository := &verificationRepositoryStub{snapshot: VerificationSnapshot{CredentialRef: "previous", MerchantPID: "merchant-previous"}}
			router := gin.New()
			RegisterRoutes(router.Group(""), NewService(repository, credentials))
			request := httptest.NewRequest(http.MethodPost, "/reader/payment/epusdt/notify", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus || response.Body.String() != test.wantBody || repository.processed != test.wantCalls {
				t.Fatalf("status=%d body=%q calls=%d", response.Code, response.Body.String(), repository.processed)
			}
		})
	}
}
