package epusdt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var clientTestNow = time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

func TestClientCreateSendsSignedFormAndParsesResponse(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		if request.Method != http.MethodPost {
			t.Errorf("method=%s", request.Method)
		}
		if got := request.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Errorf("content-type=%q", got)
		}
		if got := request.Header.Get("Accept"); got != "application/json" {
			t.Errorf("accept=%q", got)
		}
		if err := request.ParseForm(); err != nil {
			t.Error(err)
			return
		}
		want := map[string]string{
			"pid":          "merchant-current",
			"order_id":     "RC9007199254740993",
			"currency":     "usd",
			"token":        "usdt",
			"network":      "tron",
			"amount":       "1.00",
			"notify_url":   "https://reader.example/payment/callback",
			"redirect_url": "https://reader.example/recharge/result",
			"name":         "钻石充值",
		}
		for key, value := range want {
			if got := request.Form.Get(key); got != value {
				t.Errorf("form %s=%q want=%q", key, got, value)
			}
		}
		if len(request.Form) != len(want)+1 {
			t.Errorf("form keys=%v", request.Form)
		}
		if !Verify(firstFormValues(request.Form), request.Form.Get("signature"), "current-secret") {
			t.Error("request signature is invalid")
		}
		writeSuccess(writer, "RC9007199254740993", "1.00", nil)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	response, err := client.Create(context.Background(), CreateRequest{OrderID: "RC9007199254740993", Amount: "1.00"})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatalf("gateway calls=%d", calls.Load())
	}
	if response.TradeID != "trade-1" || response.OrderID != "RC9007199254740993" || response.Amount != "1.00" || response.ActualAmount != "1.00234567" || response.Currency != "usd" || response.Token != "usdt" || response.Status != 1 || response.ReceiveAddress != "TFixtureAddress" || response.PaymentURL != "https://pay.example/checkout/trade-1" || !response.ExpirationTime.Equal(clientTestNow.Add(10*time.Minute)) {
		t.Fatalf("response=%+v", response)
	}
}

func TestClientCreateDoesNotRetryRejectedRequest(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(writer, `{"status_code":422,"message":"fixture rejection","data":null}`)
	}))
	defer server.Close()

	_, err := newTestClient(t, server.URL).Create(context.Background(), validCreateRequest())
	assertGatewayError(t, err, "GATEWAY_REJECTED", FailureDefinite)
	if calls.Load() != 1 {
		t.Fatalf("gateway calls=%d", calls.Load())
	}
}

func TestClientCreateClassifiesProtocolRejectionAsDefinite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"status_code":422,"message":"fixture rejection","data":null}`)
	}))
	defer server.Close()
	_, err := newTestClient(t, server.URL).Create(context.Background(), validCreateRequest())
	assertGatewayError(t, err, "GATEWAY_REJECTED", FailureDefinite)
}

func TestClientCreateRedirectPolicy(t *testing.T) {
	t.Run("same origin", func(t *testing.T) {
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path == "/start" {
				http.Redirect(writer, request, server.URL+"/finish", http.StatusTemporaryRedirect)
				return
			}
			writeSuccess(writer, validCreateRequest().OrderID, validCreateRequest().Amount, nil)
		}))
		defer server.Close()
		if _, err := newTestClient(t, server.URL+"/start").Create(context.Background(), validCreateRequest()); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("cross host", func(t *testing.T) {
		var targetCalls atomic.Int32
		target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			targetCalls.Add(1)
		}))
		defer target.Close()
		source := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			http.Redirect(writer, request, target.URL, http.StatusTemporaryRedirect)
		}))
		defer source.Close()
		_, err := newTestClient(t, source.URL).Create(context.Background(), validCreateRequest())
		assertGatewayError(t, err, "REDIRECT_REJECTED", FailureUncertain)
		if targetCalls.Load() != 0 {
			t.Fatal("cross-host redirect target was called")
		}
	})

	t.Run("https downgrade", func(t *testing.T) {
		var targetCalls atomic.Int32
		target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			targetCalls.Add(1)
		}))
		defer target.Close()
		var source *httptest.Server
		source = httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			http.Redirect(writer, request, target.URL, http.StatusTemporaryRedirect)
		}))
		defer source.Close()
		client := newTestClient(t, source.URL)
		client.httpClient.Transport = source.Client().Transport
		_, err := client.Create(context.Background(), validCreateRequest())
		assertGatewayError(t, err, "REDIRECT_REJECTED", FailureUncertain)
		if targetCalls.Load() != 0 {
			t.Fatal("HTTPS downgrade target was called")
		}
	})

	t.Run("redirect limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			step := strings.TrimPrefix(request.URL.Path, "/")
			next := "1"
			switch step {
			case "1":
				next = "2"
			case "2":
				next = "3"
			case "3":
				next = "4"
			}
			http.Redirect(writer, request, "/"+next, http.StatusTemporaryRedirect)
		}))
		defer server.Close()
		_, err := newTestClient(t, server.URL+"/0").Create(context.Background(), validCreateRequest())
		assertGatewayError(t, err, "REDIRECT_REJECTED", FailureUncertain)
	})
}

func TestClientCreateClassifiesTransportAndResponseFailures(t *testing.T) {
	tests := []struct {
		name      string
		transport http.RoundTripper
		server    http.Handler
		wantCode  string
		wantClass FailureClass
	}{
		{name: "network", transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("fixture network failure") }), wantCode: "NETWORK_ERROR", wantClass: FailureUncertain},
		{name: "timeout", transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		}), wantCode: "REQUEST_TIMEOUT", wantClass: FailureUncertain},
		{name: "read", transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return fixtureHTTPResponse(http.StatusOK, &failingReader{}), nil
		}), wantCode: "RESPONSE_READ_ERROR", wantClass: FailureUncertain},
		{name: "invalid json", server: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(writer, "not-json") }), wantCode: "INVALID_RESPONSE", wantClass: FailureUncertain},
		{name: "unstructured HTTP error", server: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(writer, "bad gateway")
		}), wantCode: "HTTP_ERROR", wantClass: FailureUncertain},
		{name: "too large", server: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(writer, strings.Repeat("x", maxResponseBytes+1))
		}), wantCode: "RESPONSE_TOO_LARGE", wantClass: FailureUncertain},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			endpoint := "https://gateway.example/create"
			if test.server != nil {
				server := httptest.NewServer(test.server)
				defer server.Close()
				endpoint = server.URL
			}
			client := newTestClient(t, endpoint)
			if test.transport != nil {
				client.httpClient.Transport = test.transport
			}
			_, err := client.Create(context.Background(), validCreateRequest())
			assertGatewayError(t, err, test.wantCode, test.wantClass)
		})
	}
}
func TestClientCreateRejectsMismatchedSuccessFields(t *testing.T) {
	tests := []struct {
		name   string
		change func(map[string]any)
	}{
		{name: "trade id", change: func(data map[string]any) { data["trade_id"] = "" }},
		{name: "trade id whitespace", change: func(data map[string]any) { data["trade_id"] = " trade-1 " }},
		{name: "trade id length", change: func(data map[string]any) { data["trade_id"] = strings.Repeat("t", 65) }},
		{name: "order id", change: func(data map[string]any) { data["order_id"] = "RC999" }},
		{name: "amount", change: func(data map[string]any) { data["amount"] = "1.01" }},
		{name: "currency", change: func(data map[string]any) { data["currency"] = "cny" }},
		{name: "actual amount zero", change: func(data map[string]any) { data["actual_amount"] = "0.00000000" }},
		{name: "actual amount scale", change: func(data map[string]any) { data["actual_amount"] = "1.000000001" }},
		{name: "actual amount precision", change: func(data map[string]any) { data["actual_amount"] = "12345678901234567.1" }},
		{name: "address", change: func(data map[string]any) { data["receive_address"] = "" }},
		{name: "address whitespace", change: func(data map[string]any) { data["receive_address"] = " TAddress " }},
		{name: "address length", change: func(data map[string]any) { data["receive_address"] = strings.Repeat("T", 256) }},
		{name: "token", change: func(data map[string]any) { data["token"] = "btc" }},
		{name: "status", change: func(data map[string]any) { data["status"] = 2 }},
		{name: "expiration", change: func(data map[string]any) { data["expiration_time"] = clientTestNow.Unix() }},
		{name: "payment URL", change: func(data map[string]any) { data["payment_url"] = "javascript:alert(1)" }},
		{name: "payment URL length", change: func(data map[string]any) { data["payment_url"] = "https://pay.example/" + strings.Repeat("x", 1000) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writeSuccess(writer, validCreateRequest().OrderID, validCreateRequest().Amount, test.change)
			}))
			defer server.Close()
			_, err := newTestClient(t, server.URL).Create(context.Background(), validCreateRequest())
			assertGatewayError(t, err, "RESPONSE_MISMATCH", FailureUncertain)
		})
	}
}

func TestClientCreateRejectsInvalidLocalRequestBeforeSending(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer server.Close()
	client := newTestClient(t, server.URL)
	for _, request := range []CreateRequest{
		{OrderID: "", Amount: "1.00"},
		{OrderID: "MBR1", Amount: "1.00"},
		{OrderID: "RC0", Amount: "1.00"},
		{OrderID: "RC1-not-numeric", Amount: "1.00"},
		{OrderID: strings.Repeat("R", 33), Amount: "1.00"},
		{OrderID: "RC1", Amount: "1"},
		{OrderID: "RC1", Amount: "1.000"},
		{OrderID: "RC1", Amount: "0.00"},
		{OrderID: "RC1", Amount: "NaN"},
	} {
		if _, err := client.Create(context.Background(), request); err == nil {
			t.Fatalf("request accepted: %+v", request)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid requests reached gateway %d times", calls.Load())
	}
}

func TestGatewayErrorsDoNotLeakTransportDetailsOrConfiguration(t *testing.T) {
	client := newTestClient(t, "https://gateway.example/create?private=query-value")
	client.httpClient.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("transport contained current-secret and private=query-value")
	})
	_, err := client.Create(context.Background(), validCreateRequest())
	assertGatewayError(t, err, "NETWORK_ERROR", FailureUncertain)
	for _, sensitive := range []string{"current-secret", "query-value", "transport contained"} {
		if strings.Contains(err.Error(), sensitive) {
			t.Fatalf("gateway error leaked %q: %v", sensitive, err)
		}
	}
}

func newTestClient(t *testing.T, endpoint string) *Client {
	t.Helper()
	credentials, err := NewCredentialProvider("primary", "merchant-current", "current-secret", "[]")
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(Config{
		Enabled:        true,
		Credentials:    credentials,
		CreateURL:      mustURL(t, endpoint),
		NotifyURL:      mustURL(t, "https://reader.example/payment/callback"),
		RedirectURL:    mustURL(t, "https://reader.example/recharge/result"),
		ConnectTimeout: 100 * time.Millisecond,
		RequestTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	client.now = func() time.Time { return clientTestNow }
	return client
}

func validCreateRequest() CreateRequest {
	return CreateRequest{OrderID: "RC9007199254740993", Amount: "1.00"}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func assertGatewayError(t *testing.T, err error, code string, class FailureClass) {
	t.Helper()
	var gatewayError *GatewayError
	if !errors.As(err, &gatewayError) {
		t.Fatalf("error=%T %v, want GatewayError", err, err)
	}
	if gatewayError.Code != code || gatewayError.Class != class || len(gatewayError.Summary) == 0 || len(gatewayError.Summary) > 500 {
		t.Fatalf("gateway error=%+v", gatewayError)
	}
}

func firstFormValues(values url.Values) map[string]string {
	result := make(map[string]string, len(values))
	for key := range values {
		result[key] = values.Get(key)
	}
	return result
}

func fixtureHTTPResponse(status int, body io.ReadCloser) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: body}
}

type failingReader struct{}

func (*failingReader) Read([]byte) (int, error) { return 0, errors.New("fixture read failure") }
func (*failingReader) Close() error             { return nil }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func writeSuccess(writer http.ResponseWriter, orderID, amount string, change func(map[string]any)) {
	data := map[string]any{
		"trade_id":        "trade-1",
		"order_id":        orderID,
		"amount":          amount,
		"currency":        "usd",
		"actual_amount":   "1.00234567",
		"receive_address": "TFixtureAddress",
		"token":           "usdt",
		"status":          1,
		"expiration_time": clientTestNow.Add(10 * time.Minute).Unix(),
		"payment_url":     "https://pay.example/checkout/trade-1",
	}
	if change != nil {
		change(data)
	}
	writer.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(writer, `{"status_code":200,"data":{"trade_id":%q,"order_id":%q,"amount":%q,"currency":%q,"actual_amount":%q,"receive_address":%q,"token":%q,"status":%v,"expiration_time":%v,"payment_url":%q}}`,
		data["trade_id"], data["order_id"], data["amount"], data["currency"], data["actual_amount"], data["receive_address"], data["token"], data["status"], data["expiration_time"], data["payment_url"])
}
