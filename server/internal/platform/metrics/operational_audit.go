package metrics

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/reconcile"
	"github.com/prometheus/client_golang/prometheus"
)

const operationalAuditInterval = 5 * time.Minute

type operationalAuditSnapshot struct {
	migrationErrors   float64
	unhealthyObjects  float64
	financeMismatches float64
	collectionSuccess float64
	lastSuccess       float64
}

type operationalAuditCollector struct {
	mu       sync.RWMutex
	running  bool
	lastRun  time.Time
	snapshot operationalAuditSnapshot
	audit    func(context.Context) (operationalAuditSnapshot, error)

	migrationErrors   *prometheus.Desc
	unhealthyObjects  *prometheus.Desc
	financeMismatches *prometheus.Desc
	collectionSuccess *prometheus.Desc
	lastSuccess       *prometheus.Desc
}

func newOperationalAuditCollector(db *sql.DB) *operationalAuditCollector {
	collector := &operationalAuditCollector{
		migrationErrors:   prometheus.NewDesc("moonbook_migration_open_errors", "Unresolved legacy migration errors.", nil, nil),
		unhealthyObjects:  prometheus.NewDesc("moonbook_object_unhealthy_records", "Failed or stale transitional content objects.", nil, nil),
		financeMismatches: prometheus.NewDesc("moonbook_finance_reconcile_mismatches", "Current full financial reconciliation mismatches.", nil, nil),
		collectionSuccess: prometheus.NewDesc("moonbook_operational_audit_collection_success", "Whether the latest operational audit succeeded.", nil, nil),
		lastSuccess:       prometheus.NewDesc("moonbook_operational_audit_last_success_timestamp_seconds", "Unix timestamp of the latest successful operational audit.", nil, nil),
	}
	collector.audit = func(ctx context.Context) (operationalAuditSnapshot, error) {
		var snapshot operationalAuditSnapshot
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM migration_errors WHERE resolved_at IS NULL`).Scan(&snapshot.migrationErrors); err != nil {
			return snapshot, err
		}
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM novel_objects WHERE state='failed' OR (state IN ('uploading','deleting') AND created_at < now()-interval '15 minutes')`).Scan(&snapshot.unhealthyObjects); err != nil {
			return snapshot, err
		}
		report, err := reconcile.Full(ctx, db)
		if err != nil {
			return snapshot, err
		}
		snapshot.financeMismatches = float64(len(report.Mismatches))
		return snapshot, nil
	}
	return collector
}

func (collector *operationalAuditCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.migrationErrors
	ch <- collector.unhealthyObjects
	ch <- collector.financeMismatches
	ch <- collector.collectionSuccess
	ch <- collector.lastSuccess
}

func (collector *operationalAuditCollector) Collect(ch chan<- prometheus.Metric) {
	collector.startRefresh()
	collector.mu.RLock()
	snapshot := collector.snapshot
	collector.mu.RUnlock()
	ch <- prometheus.MustNewConstMetric(collector.migrationErrors, prometheus.GaugeValue, snapshot.migrationErrors)
	ch <- prometheus.MustNewConstMetric(collector.unhealthyObjects, prometheus.GaugeValue, snapshot.unhealthyObjects)
	ch <- prometheus.MustNewConstMetric(collector.financeMismatches, prometheus.GaugeValue, snapshot.financeMismatches)
	ch <- prometheus.MustNewConstMetric(collector.collectionSuccess, prometheus.GaugeValue, snapshot.collectionSuccess)
	ch <- prometheus.MustNewConstMetric(collector.lastSuccess, prometheus.GaugeValue, snapshot.lastSuccess)
}

func (collector *operationalAuditCollector) startRefresh() {
	collector.mu.Lock()
	if collector.running || (!collector.lastRun.IsZero() && time.Since(collector.lastRun) < operationalAuditInterval) {
		collector.mu.Unlock()
		return
	}
	collector.running = true
	collector.lastRun = time.Now()
	collector.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		collector.refresh(ctx, time.Now().UTC())
	}()
}

func (collector *operationalAuditCollector) refresh(ctx context.Context, now time.Time) {
	snapshot, err := collector.audit(ctx)
	collector.mu.Lock()
	defer collector.mu.Unlock()
	collector.running = false
	if err != nil {
		collector.snapshot.collectionSuccess = 0
		return
	}
	snapshot.collectionSuccess = 1
	snapshot.lastSuccess = float64(now.Unix())
	collector.snapshot = snapshot
}
