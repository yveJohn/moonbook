//go:build integration

package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/epusdt"
	"github.com/gin-gonic/gin"
)

type retryOnceProcessor struct {
	delegate CallbackProcessor
	calls    int
}

func (processor *retryOnceProcessor) ProcessAttempt(ctx context.Context, attemptID int64, callback Callback) error {
	processor.calls++
	if processor.calls == 1 {
		return newProcessingError(FailureDependency, readerDependencyUnavailable)
	}
	return processor.delegate.ProcessAttempt(ctx, attemptID, callback)
}

func TestCallbackHTTPRetryCreatesIndependentAttemptsAndThenSucceeds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fixture := newLateCallbackFixture(t, "pending", 500)
	repository := fixture.repository()
	credentials, err := epusdt.NewCredentialProvider("primary", "merchant", "secret", "[]")
	if err != nil {
		t.Fatal(err)
	}
	processor := &retryOnceProcessor{delegate: NewService(repository, credentials)}
	router := gin.New()
	requestPrefix := "retry-" + strings.ToLower(strings.ReplaceAll(fixture.orderNo, "_", "-"))
	if len(requestPrefix) > 50 {
		requestPrefix = requestPrefix[:50]
	}
	metadata := func(context.Context) RequestMetadata {
		return RequestMetadata{RequestID: requestPrefix, TraceID: "retry-trace", ClientIP: "127.0.0.1"}
	}
	RegisterRoutes(router.Group(""), NewHandler(processor, repository, metadata, nil))
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	callback := fixture.signedCallback("merchant", "secret")
	payload, err := json.Marshal(callback.Fields)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = fixture.db.ExecContext(context.Background(), `DELETE FROM reader_payment_callback_logs WHERE request_id LIKE $1`, requestPrefix+"%")
	})

	statuses := make([]int, 0, 2)
	bodies := make([]string, 0, 2)
	for attempt := 1; attempt <= 2; attempt++ {
		request, err := http.NewRequest(http.MethodPost, server.URL+"/reader/payment/epusdt/notify", bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Request-Id", requestPrefix+"-"+strconv.Itoa(attempt))
		request.Header.Set("X-Trace-Id", "retry-trace-"+strconv.Itoa(attempt))
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		statuses = append(statuses, response.StatusCode)
		bodies = append(bodies, string(body))
		if response.StatusCode == http.StatusOK && string(body) == "success" {
			break
		}
	}
	if len(statuses) != 2 || statuses[0] != http.StatusServiceUnavailable || bodies[0] != "fail" || statuses[1] != http.StatusOK || bodies[1] != "success" {
		t.Fatalf("statuses=%v bodies=%v", statuses, bodies)
	}

	var attempts, failed, success, unsafeSnapshots int
	if err := fixture.db.QueryRowContext(fixture.ctx, `
SELECT count(*),
       count(*) FILTER (WHERE processing_result='failed' AND failure_code='DEPENDENCY_FAILED'),
       count(*) FILTER (WHERE processing_result='success'),
       count(*) FILTER (WHERE payload_snapshot ? 'pid' OR payload_snapshot ? 'signature')
FROM reader_payment_callback_logs
WHERE request_id LIKE $1`, requestPrefix+"%").Scan(&attempts, &failed, &success, &unsafeSnapshots); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || failed != 1 || success != 1 || unsafeSnapshots != 0 || processor.calls != 2 {
		t.Fatalf("attempts=%d failed=%d success=%d unsafe=%d calls=%d", attempts, failed, success, unsafeSnapshots, processor.calls)
	}
}
