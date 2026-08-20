package metrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestPaymentCallbackObserverUsesClosedLowCardinalityMetrics(t *testing.T) {
	metrics := New(nil)
	for _, result := range []string{"received", "success", "idempotent", "rejected", "failed"} {
		metrics.ObserveCallbackAttempt(result)
	}
	metrics.ObserveCallbackAttempt("ORDER-9223372036854775807")
	metrics.ObserveCallbackResponse(http.StatusServiceUnavailable)
	metrics.ObserveCallbackResponse(http.StatusBadRequest)

	body := scrapeMetrics(t, metrics)
	for _, want := range []string{
		`moonbook_payment_callback_attempts_total{result="received"} 1`,
		`moonbook_payment_callback_attempts_total{result="success"} 1`,
		`moonbook_payment_callback_attempts_total{result="idempotent"} 1`,
		`moonbook_payment_callback_attempts_total{result="rejected"} 1`,
		`moonbook_payment_callback_attempts_total{result="failed"} 1`,
		`moonbook_payment_callback_responses_503_total 1`,
		`moonbook_payment_callback_idempotent_retries_total 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics missing %q\n%s", want, body)
		}
	}
	if strings.Contains(body, "ORDER-9223372036854775807") {
		t.Fatal("callback metrics contain a high-cardinality result")
	}
}

func TestPaymentCallbackCollectorExposesStaleCountAndSanitizedFailure(t *testing.T) {
	collector := newPaymentCallbackCollector(nil, 5*time.Minute)
	collector.queryStale = func(ctx context.Context, staleBefore time.Time) (float64, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("collector query has no deadline")
		}
		if delta := time.Since(staleBefore); delta < 4*time.Minute || delta > 6*time.Minute {
			t.Fatalf("stale threshold delta=%s", delta)
		}
		return 3, nil
	}
	body := gatherCollector(t, collector)
	for _, want := range []string{
		"moonbook_payment_callback_stale_received 3",
		"moonbook_payment_callback_metrics_collection_success 1",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics missing %q\n%s", want, body)
		}
	}

	collector.queryStale = func(context.Context, time.Time) (float64, error) {
		return 0, errors.New("postgres secret order 9223372036854775807")
	}
	body = gatherCollector(t, collector)
	if !strings.Contains(body, "moonbook_payment_callback_metrics_collection_success 0") {
		t.Fatalf("collection failure metric missing\n%s", body)
	}
	for _, forbidden := range []string{"postgres secret", "9223372036854775807", "moonbook_payment_callback_stale_received"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("failed collection exposed %q\n%s", forbidden, body)
		}
	}
}

func scrapeMetrics(t *testing.T, metrics *Metrics) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	metrics.Handler("test-token").ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("metrics status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	return recorder.Body.String()
}

func gatherCollector(t *testing.T, collector prometheus.Collector) string {
	t.Helper()
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)
	recorder := httptest.NewRecorder()
	promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("collector status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	return recorder.Body.String()
}
