package adminrechargeorder

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuildSyncURLUsesEscapedTradeID(t *testing.T) {
	got, err := buildSyncURL("https://pay.example/pay/check-status/{trade_id}", "trade/with space")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, got, nil)
	if request.URL.EscapedPath() != "/pay/check-status/trade%2Fwith%20space" {
		t.Fatalf("escaped path=%q", request.URL.EscapedPath())
	}
	for _, template := range []string{"", "https://pay.example/status", "https://pay.example/{trade_id}/{trade_id}"} {
		if _, err = buildSyncURL(template, "trade-1"); err == nil {
			t.Fatalf("buildSyncURL(%q) should fail", template)
		}
	}
}

func TestParseGMStatusValidatesEnvelopeAndOrder(t *testing.T) {
	status, err := parseGMStatus([]byte(`{"status_code":200,"message":"success","data":{"trade_id":"trade-1","status":2}}`), "trade-1")
	if err != nil || status != 2 {
		t.Fatalf("status=%d err=%v", status, err)
	}
	for _, raw := range []string{
		`{"status_code":200,"data":{"trade_id":"other","status":2}}`,
		`{"status_code":500,"data":{"trade_id":"trade-1","status":2}}`,
		`{"status_code":200,"data":{"trade_id":"trade-1","status":5}}`,
		`not-json`,
	} {
		if _, err = parseGMStatus([]byte(raw), "trade-1"); err == nil {
			t.Fatalf("parseGMStatus(%q) should fail", raw)
		}
	}
}

func TestSyncOutcomeNeverSettlesUnsignedPaidStatus(t *testing.T) {
	tests := []struct {
		gatewayStatus int
		result        string
		localStatus   string
		clearActive   bool
	}{
		{1, "sync_pending", "pending", false},
		{2, "paid_no_callback", "callback_exception", false},
		{3, "sync_expired", "expired", true},
		{4, "sync_select", "gateway_unknown", false},
	}
	for _, test := range tests {
		result, _, localStatus, clearActive := syncOutcome(test.gatewayStatus)
		if result != test.result || localStatus != test.localStatus || clearActive != test.clearActive || localStatus == "paid" {
			t.Fatalf("gateway status %d mapped to result=%q status=%q clear=%v", test.gatewayStatus, result, localStatus, clearActive)
		}
	}
}
