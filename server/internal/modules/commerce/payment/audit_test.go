package payment

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewAttemptStartHashesOnlyCapturedPayloadAndSourceIP(t *testing.T) {
	payload := []byte(`{"pid":"merchant","signature":"secret-signature","order_id":"RC1"}`)
	requestedAt := time.Date(2026, 8, 20, 7, 30, 0, 0, time.UTC)
	start, err := NewAttemptStart(payload, false, "request-1", "trace-1", "203.0.113.9", requestedAt)
	if err != nil {
		t.Fatal(err)
	}
	payloadHash := sha256.Sum256(payload)
	ipHash := sha256.Sum256([]byte("203.0.113.9"))
	if start.PayloadHash != hex.EncodeToString(payloadHash[:]) || start.SourceIPSHA256 != hex.EncodeToString(ipHash[:]) {
		t.Fatalf("unexpected hashes: %+v", start)
	}
	if start.PayloadBytes != len(payload) || start.PayloadTruncated || !start.RequestTime.Equal(requestedAt) {
		t.Fatalf("unexpected metadata: %+v", start)
	}
	if strings.Contains(start.PayloadHash, "merchant") || strings.Contains(start.SourceIPSHA256, "203.0.113.9") {
		t.Fatalf("audit metadata contains plaintext: %+v", start)
	}
}

func TestNewAttemptStartValidatesBoundaries(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name      string
		payload   []byte
		truncated bool
		requestID string
		traceID   string
		when      time.Time
		wantErr   bool
	}{
		{name: "empty payload is hashable", requestID: "request", traceID: "trace", when: now},
		{name: "maximum captured payload", payload: make([]byte, 16385), truncated: true, requestID: "request", traceID: "trace", when: now},
		{name: "oversized capture", payload: make([]byte, 16386), truncated: true, requestID: "request", traceID: "trace", when: now, wantErr: true},
		{name: "truncated short capture", payload: make([]byte, 16384), truncated: true, requestID: "request", traceID: "trace", when: now, wantErr: true},
		{name: "missing request id", payload: []byte("{}"), traceID: "trace", when: now, wantErr: true},
		{name: "invalid trace id", payload: []byte("{}"), requestID: "request", traceID: "bad trace", when: now, wantErr: true},
		{name: "missing request time", payload: []byte("{}"), requestID: "request", traceID: "trace", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewAttemptStart(test.payload, test.truncated, test.requestID, test.traceID, "127.0.0.1", test.when)
			if (err != nil) != test.wantErr {
				t.Fatalf("err=%v wantErr=%t", err, test.wantErr)
			}
		})
	}
}

func TestSnapshotFromCallbackUsesExplicitWhitelist(t *testing.T) {
	callback := Callback{
		OrderNo: "RC1", TradeID: "trade-1", Amount: "2.00", ActualAmount: "1.9999",
		ReceiveAddress: "T-address", Token: "usdt", TransactionID: "tx-1", Status: 2,
		PID: "merchant-secret", Signature: "full-signature",
		Fields: map[string]string{"unknown": "must-not-survive", "nested": `{"secret":"value"}`},
	}
	snapshot := SnapshotFromCallback(callback)
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, want := range []string{`"order_id":"RC1"`, `"trade_id":"trade-1"`, `"amount":"2.00"`, `"actual_amount":"1.9999"`, `"receive_address":"T-address"`, `"token":"usdt"`, `"block_transaction_id":"tx-1"`, `"status":"2"`} {
		if !strings.Contains(text, want) {
			t.Errorf("snapshot missing %s: %s", want, text)
		}
	}
	for _, forbidden := range []string{"signature", "pid", "merchant-secret", "full-signature", "unknown", "nested", "must-not-survive"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("snapshot contains %q: %s", forbidden, text)
		}
	}
}

func TestAttemptCompletionValidatesClosedOutcomes(t *testing.T) {
	snapshot := SnapshotFromCallback(Callback{OrderNo: "RC1", TradeID: "trade", Status: 2})
	orderID := int64(9007199254740993)
	tests := []struct {
		name       string
		completion AttemptCompletion
		wantErr    bool
	}{
		{name: "rejected format", completion: AttemptCompletion{Result: ResultRejected, FailureCode: FailureInvalidPayload, ResponseStatus: 400}},
		{name: "unauthorized signature", completion: AttemptCompletion{Result: ResultRejected, FailureCode: FailureSignatureInvalid, ResponseStatus: 401, Snapshot: &snapshot}},
		{name: "failed dependency", completion: AttemptCompletion{Result: ResultFailed, FailureCode: FailureDependency, ResponseStatus: 503}},
		{name: "success", completion: AttemptCompletion{Result: ResultSuccess, ResponseStatus: 200, SignatureValid: true, OrderID: &orderID, Snapshot: &snapshot}},
		{name: "idempotent", completion: AttemptCompletion{Result: ResultIdempotent, ResponseStatus: 200, SignatureValid: true, OrderID: &orderID, Snapshot: &snapshot}},
		{name: "received is not terminal", completion: AttemptCompletion{Result: ResultReceived}, wantErr: true},
		{name: "unknown result", completion: AttemptCompletion{Result: "unknown", FailureCode: FailureDependency, ResponseStatus: 503}, wantErr: true},
		{name: "unknown failure code", completion: AttemptCompletion{Result: ResultFailed, FailureCode: "DATABASE_MESSAGE", ResponseStatus: 503}, wantErr: true},
		{name: "wrong rejected status", completion: AttemptCompletion{Result: ResultRejected, FailureCode: FailureSignatureInvalid, ResponseStatus: 400}, wantErr: true},
		{name: "failure has no code", completion: AttemptCompletion{Result: ResultFailed, ResponseStatus: 503}, wantErr: true},
		{name: "success has failure code", completion: AttemptCompletion{Result: ResultSuccess, FailureCode: FailureDependency, ResponseStatus: 200, SignatureValid: true}, wantErr: true},
		{name: "success without verified signature", completion: AttemptCompletion{Result: ResultSuccess, ResponseStatus: 200}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateAttemptCompletion(test.completion)
			if (err != nil) != test.wantErr {
				t.Fatalf("err=%v wantErr=%t completion=%+v", err, test.wantErr, test.completion)
			}
		})
	}
}

func TestFailureReasonIsFixedAndClosed(t *testing.T) {
	for code, want := range map[FailureCode]string{
		FailurePayloadTooLarge:   "Callback payload exceeded the size limit",
		FailureRequestRead:       "Callback request could not be read",
		FailureInvalidPayload:    "Callback payload was invalid",
		FailureUnknownOrder:      "Recharge order was not found",
		FailureUnknownCredential: "Callback credential was not recognized",
		FailurePIDMismatch:       "Callback merchant identity did not match",
		FailureSignatureInvalid:  "Callback signature validation failed",
		FailureSnapshotMismatch:  "Callback did not match the order snapshot",
		FailureReplay:            "Callback reused a payment identifier",
		FailureDependency:        "Callback dependency was unavailable",
		FailureTransaction:       "Callback transaction could not be committed",
		FailurePanic:             "Callback processing stopped unexpectedly",
	} {
		if got, ok := FailureReason(code); !ok || got != want {
			t.Errorf("code=%s reason=%q ok=%t want=%q", code, got, ok, want)
		}
	}
	if reason, ok := FailureReason("SQLSTATE-SECRET"); ok || reason != "" {
		t.Fatalf("unknown failure reason=%q ok=%t", reason, ok)
	}
}

func TestAuditRepositoryRejectsNilDatabase(t *testing.T) {
	completion := AttemptCompletion{Result: ResultFailed, FailureCode: FailureDependency, ResponseStatus: 503}
	if _, err := (SQLRepository{}).BeginAttempt(t.Context(), AttemptStart{}); !errors.Is(err, ErrInvalidAttempt) {
		t.Fatalf("begin err=%v", err)
	}
	if err := (SQLRepository{}).FinalizeAttempt(t.Context(), 1, completion); !errors.Is(err, ErrInvalidAttempt) {
		t.Fatalf("finalize err=%v", err)
	}
}
