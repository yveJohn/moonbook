package metrics

import (
	"context"
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const defaultCallbackStaleAfter = 5 * time.Minute

var callbackResults = map[string]struct{}{
	"received": {}, "success": {}, "idempotent": {}, "rejected": {}, "failed": {},
}

type paymentCallbackCollector struct {
	staleAfter       time.Duration
	staleReceived    *prometheus.Desc
	collectionStatus *prometheus.Desc
	queryStale       func(context.Context, time.Time) (float64, error)
}

func newPaymentCallbackCollector(db *sql.DB, staleAfter time.Duration) *paymentCallbackCollector {
	if staleAfter <= 0 {
		staleAfter = defaultCallbackStaleAfter
	}
	collector := &paymentCallbackCollector{
		staleAfter: staleAfter,
		staleReceived: prometheus.NewDesc(
			"moonbook_payment_callback_stale_received",
			"Current runtime callback attempts still received beyond the configured threshold.",
			nil, nil,
		),
		collectionStatus: prometheus.NewDesc(
			"moonbook_payment_callback_metrics_collection_success",
			"Whether the latest payment callback database metrics collection succeeded.",
			nil, nil,
		),
	}
	collector.queryStale = func(ctx context.Context, staleBefore time.Time) (float64, error) {
		var count float64
		err := db.QueryRowContext(ctx, `
SELECT count(*) FROM reader_payment_callback_logs
WHERE source_type='runtime' AND processing_result='received' AND request_time < $1`, staleBefore).Scan(&count)
		return count, err
	}
	return collector
}

func (collector *paymentCallbackCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.staleReceived
	ch <- collector.collectionStatus
}

func (collector *paymentCallbackCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	count, err := collector.queryStale(ctx, time.Now().UTC().Add(-collector.staleAfter))
	if err != nil {
		ch <- prometheus.MustNewConstMetric(collector.collectionStatus, prometheus.GaugeValue, 0)
		return
	}
	ch <- prometheus.MustNewConstMetric(collector.staleReceived, prometheus.GaugeValue, count)
	ch <- prometheus.MustNewConstMetric(collector.collectionStatus, prometheus.GaugeValue, 1)
}
