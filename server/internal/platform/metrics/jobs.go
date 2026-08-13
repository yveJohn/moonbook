package metrics

import (
	"context"
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type jobCollector struct {
	db             *sql.DB
	jobs           *prometheus.Desc
	overdueLeases  *prometheus.Desc
	collectFailure *prometheus.Desc
}

func newJobCollector(db *sql.DB) *jobCollector {
	return &jobCollector{
		db: db,
		jobs: prometheus.NewDesc(
			"moonbook_platform_jobs",
			"Current persisted jobs by module, type and status.",
			[]string{"module", "job_type", "status"}, nil,
		),
		overdueLeases: prometheus.NewDesc(
			"moonbook_platform_job_overdue_leases",
			"Running jobs whose lease has expired.",
			nil, nil,
		),
		collectFailure: prometheus.NewDesc(
			"moonbook_platform_job_metrics_collection_success",
			"Whether the latest platform job metrics collection succeeded.",
			nil, nil,
		),
	}
}

func (collector *jobCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.jobs
	ch <- collector.overdueLeases
	ch <- collector.collectFailure
}

func (collector *jobCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	rows, err := collector.db.QueryContext(ctx, `
		SELECT module, job_type, status, count(*)
		FROM platform_jobs
		GROUP BY module, job_type, status
		ORDER BY module, job_type, status`)
	if err != nil {
		ch <- prometheus.NewInvalidMetric(collector.jobs, err)
		ch <- prometheus.MustNewConstMetric(collector.collectFailure, prometheus.GaugeValue, 0)
		return
	}
	for rows.Next() {
		var module, jobType, status string
		var count float64
		if err := rows.Scan(&module, &jobType, &status, &count); err != nil {
			rows.Close()
			ch <- prometheus.NewInvalidMetric(collector.jobs, err)
			ch <- prometheus.MustNewConstMetric(collector.collectFailure, prometheus.GaugeValue, 0)
			return
		}
		ch <- prometheus.MustNewConstMetric(collector.jobs, prometheus.GaugeValue, count, module, jobType, status)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		ch <- prometheus.NewInvalidMetric(collector.jobs, err)
		ch <- prometheus.MustNewConstMetric(collector.collectFailure, prometheus.GaugeValue, 0)
		return
	}
	if err := rows.Close(); err != nil {
		ch <- prometheus.NewInvalidMetric(collector.jobs, err)
		ch <- prometheus.MustNewConstMetric(collector.collectFailure, prometheus.GaugeValue, 0)
		return
	}
	var overdue float64
	if err := collector.db.QueryRowContext(ctx, `
		SELECT count(*) FROM platform_jobs
		WHERE status = 'running' AND lease_expires_at < now()`).Scan(&overdue); err != nil {
		ch <- prometheus.NewInvalidMetric(collector.overdueLeases, err)
		ch <- prometheus.MustNewConstMetric(collector.collectFailure, prometheus.GaugeValue, 0)
		return
	}
	ch <- prometheus.MustNewConstMetric(collector.overdueLeases, prometheus.GaugeValue, overdue)
	ch <- prometheus.MustNewConstMetric(collector.collectFailure, prometheus.GaugeValue, 1)
}
