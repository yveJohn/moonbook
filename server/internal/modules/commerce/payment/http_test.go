package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type callbackProcessorStub struct {
	err      error
	panicNow bool
	calls    int
	attempt  int64
}

func (stub *callbackProcessorStub) ProcessAttempt(_ context.Context, attemptID int64, _ Callback) error {
	stub.calls++
	stub.attempt = attemptID
	if stub.panicNow {
		panic("sensitive panic value")
	}
	return stub.err
}

type callbackAuditStub struct {
	beginID      int64
	beginErr     error
	finalizeErr  error
	starts       []AttemptStart
	completions  []AttemptCompletion
	finalizedIDs []int64
}

func (stub *callbackAuditStub) BeginAttempt(_ context.Context, start AttemptStart) (int64, error) {
	stub.starts = append(stub.starts, start)
	return stub.beginID, stub.beginErr
}

func (stub *callbackAuditStub) FinalizeAttempt(_ context.Context, id int64, completion AttemptCompletion) error {
	stub.finalizedIDs = append(stub.finalizedIDs, id)
	stub.completions = append(stub.completions, completion)
	return stub.finalizeErr
}

type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, errors.New("sensitive read error") }
func (failingBody) Close() error             { return nil }

func testRequestMetadata(context.Context) RequestMetadata {
	return RequestMetadata{RequestID: "request-http-test", TraceID: "trace-http-test", ClientIP: "203.0.113.30"}
}

func TestReadCallbackBodyUsesOneByteOverflowSentinel(t *testing.T) {
	tests := []struct {
		name          string
		reader        io.Reader
		wantBytes     int
		wantTruncated bool
		wantErr       bool
	}{
		{name: "empty", reader: strings.NewReader(""), wantBytes: 0},
		{name: "exact limit", reader: strings.NewReader(strings.Repeat("a", 16384)), wantBytes: 16384},
		{name: "one byte over", reader: strings.NewReader(strings.Repeat("a", 16385)), wantBytes: 16385, wantTruncated: true},
		{name: "larger body", reader: strings.NewReader(strings.Repeat("a", 20000)), wantBytes: 16385, wantTruncated: true},
		{name: "read failure", reader: failingBody{}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body, truncated, err := readCallbackBody(test.reader)
			if len(body) != test.wantBytes || truncated != test.wantTruncated || (err != nil) != test.wantErr {
				t.Fatalf("bytes=%d truncated=%t err=%v", len(body), truncated, err)
			}
		})
	}
}

func TestParseCallbackRejectsNonScalarAndTrailingValues(t *testing.T) {
	valid := `{"pid":"merchant","trade_id":"trade","order_id":"RC1","amount":"2.00","actual_amount":"2.00","receive_address":"T-address","token":"usdt","block_transaction_id":"tx","status":2,"signature":"signature"}`
	if callback, err := parseCallback([]byte(valid)); err != nil || callback.Status != 2 || callback.OrderNo != "RC1" {
		t.Fatalf("callback=%+v err=%v", callback, err)
	}
	for _, payload := range []string{
		"", "null", valid + `{}`,
		strings.Replace(valid, `"pid":"merchant"`, `"pid":null`, 1),
		strings.Replace(valid, `"pid":"merchant"`, `"pid":true`, 1),
		strings.Replace(valid, `"pid":"merchant"`, `"pid":[]`, 1),
		strings.Replace(valid, `"pid":"merchant"`, `"pid":{}`, 1),
	} {
		if callback, err := parseCallback([]byte(payload)); err == nil {
			t.Fatalf("accepted payload=%q callback=%+v", payload, callback)
		}
	}
}

func TestCallbackHTTPAuditsBeforeParsingAndProcessing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fields := map[string]string{
		"pid": "merchant", "trade_id": "trade", "order_id": "RC1", "amount": "2.00", "actual_amount": "2.00",
		"receive_address": "T-address", "token": "usdt", "block_transaction_id": "tx", "status": "2",
	}
	fields["signature"] = "test-signature"
	validBody, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name         string
		body         io.Reader
		beginErr     error
		processorErr error
		wantStatus   int
		wantBody     string
		wantCalls    int
		wantResult   AttemptResult
		wantCode     FailureCode
		finalizeErr  error
		panicNow     bool
	}{
		{name: "success", body: bytes.NewReader(validBody), wantStatus: http.StatusOK, wantBody: "success", wantCalls: 1},
		{name: "invalid payload", body: strings.NewReader("not-json"), wantStatus: http.StatusBadRequest, wantBody: "fail", wantResult: ResultRejected, wantCode: FailureInvalidPayload},
		{name: "payload too large", body: strings.NewReader(strings.Repeat("x", 16385)), wantStatus: http.StatusBadRequest, wantBody: "fail", wantResult: ResultRejected, wantCode: FailurePayloadTooLarge},
		{name: "read failure", body: failingBody{}, wantStatus: http.StatusServiceUnavailable, wantBody: "fail", wantResult: ResultFailed, wantCode: FailureRequestRead},
		{name: "signature failure", body: bytes.NewReader(validBody), processorErr: newProcessingError(FailureSignatureInvalid, ErrUnauthorized), wantStatus: http.StatusUnauthorized, wantBody: "fail", wantCalls: 1, wantResult: ResultRejected, wantCode: FailureSignatureInvalid},
		{name: "snapshot mismatch", body: bytes.NewReader(validBody), processorErr: newProcessingError(FailureSnapshotMismatch, ErrRejected), wantStatus: http.StatusBadRequest, wantBody: "fail", wantCalls: 1, wantResult: ResultRejected, wantCode: FailureSnapshotMismatch},
		{name: "dependency failure", body: bytes.NewReader(validBody), processorErr: newProcessingError(FailureDependency, readerDependencyUnavailable), wantStatus: http.StatusServiceUnavailable, wantBody: "fail", wantCalls: 1, wantResult: ResultFailed, wantCode: FailureDependency},
		{name: "finalize failure", body: strings.NewReader("not-json"), finalizeErr: errors.New("database unavailable"), wantStatus: http.StatusServiceUnavailable, wantBody: "fail", wantResult: ResultRejected, wantCode: FailureInvalidPayload},
		{name: "panic", body: bytes.NewReader(validBody), panicNow: true, wantStatus: http.StatusServiceUnavailable, wantBody: "fail", wantCalls: 1, wantResult: ResultFailed, wantCode: FailurePanic},
		{name: "begin failure", body: bytes.NewReader(validBody), beginErr: errors.New("database unavailable"), wantStatus: http.StatusServiceUnavailable, wantBody: "fail"},
	} {
		t.Run(test.name, func(t *testing.T) {
			processor := &callbackProcessorStub{err: test.processorErr, panicNow: test.panicNow}
			audit := &callbackAuditStub{beginID: 9223372036854775000, beginErr: test.beginErr, finalizeErr: test.finalizeErr}
			router := gin.New()
			RegisterRoutes(router.Group(""), NewHandler(processor, audit, testRequestMetadata, nil))
			request := httptest.NewRequest(http.MethodPost, "/reader/payment/epusdt/notify", test.body)
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Request-Id", "request-http-test")
			request.Header.Set("X-Trace-Id", "trace-http-test")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus || response.Body.String() != test.wantBody || processor.calls != test.wantCalls {
				t.Fatalf("status=%d body=%q calls=%d", response.Code, response.Body.String(), processor.calls)
			}
			if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
				t.Fatalf("content-type=%q", contentType)
			}
			if len(audit.starts) != 1 || audit.starts[0].RequestID != "request-http-test" || audit.starts[0].TraceID != "trace-http-test" || audit.starts[0].RequestTime.After(time.Now()) {
				t.Fatalf("starts=%+v", audit.starts)
			}
			if test.wantResult == "" {
				if len(audit.completions) != 0 {
					t.Fatalf("unexpected completions=%+v", audit.completions)
				}
			} else if len(audit.completions) != 1 || audit.completions[0].Result != test.wantResult || audit.completions[0].FailureCode != test.wantCode {
				t.Fatalf("completions=%+v", audit.completions)
			}
		})
	}
}
