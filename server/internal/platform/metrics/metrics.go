package metrics

import (
	"crypto/subtle"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry                  *prometheus.Registry
	requestsTotal             *prometheus.CounterVec
	duration                  *prometheus.HistogramVec
	inFlight                  prometheus.Gauge
	callbackAttempts          *prometheus.CounterVec
	callbackResponses503      prometheus.Counter
	callbackIdempotentRetries prometheus.Counter
}

func New(db *sql.DB, callbackStaleAfter ...time.Duration) *Metrics {
	registry := prometheus.NewRegistry()
	requestsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "moonbook",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total HTTP requests by method, route template and status.",
	}, []string{"method", "route", "status"})
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "moonbook",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "HTTP request duration by method and route template.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"method", "route"})
	inFlight := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "moonbook",
		Subsystem: "http",
		Name:      "requests_in_flight",
		Help:      "Current number of HTTP requests being handled.",
	})
	callbackAttempts := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "moonbook", Subsystem: "payment", Name: "callback_attempts_total",
		Help: "Payment callback audit state transitions by closed result.",
	}, []string{"result"})
	callbackResponses503 := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "moonbook", Subsystem: "payment", Name: "callback_responses_503_total",
		Help: "Payment callback HTTP 503 responses.",
	})
	callbackIdempotentRetries := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "moonbook", Subsystem: "payment", Name: "callback_idempotent_retries_total",
		Help: "Payment callback retries that completed as valid idempotent attempts.",
	})
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(requestsTotal, duration, inFlight, callbackAttempts, callbackResponses503, callbackIdempotentRetries)
	if db != nil {
		registry.MustRegister(newJobCollector(db))
		staleAfter := defaultCallbackStaleAfter
		if len(callbackStaleAfter) > 0 && callbackStaleAfter[0] > 0 {
			staleAfter = callbackStaleAfter[0]
		}
		registry.MustRegister(newPaymentCallbackCollector(db, staleAfter))
	}
	return &Metrics{
		registry: registry, requestsTotal: requestsTotal, duration: duration, inFlight: inFlight,
		callbackAttempts: callbackAttempts, callbackResponses503: callbackResponses503,
		callbackIdempotentRetries: callbackIdempotentRetries,
	}
}

func (metrics *Metrics) ObserveCallbackAttempt(result string) {
	if metrics == nil {
		return
	}
	if _, ok := callbackResults[result]; !ok {
		return
	}
	metrics.callbackAttempts.WithLabelValues(result).Inc()
	if result == "idempotent" {
		metrics.callbackIdempotentRetries.Inc()
	}
}

func (metrics *Metrics) ObserveCallbackResponse(status int) {
	if metrics != nil && status == http.StatusServiceUnavailable {
		metrics.callbackResponses503.Inc()
	}
}

func (metrics *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		metrics.inFlight.Inc()
		defer metrics.inFlight.Dec()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		method := c.Request.Method
		metrics.requestsTotal.WithLabelValues(method, route, strconv.Itoa(c.Writer.Status())).Inc()
		metrics.duration.WithLabelValues(method, route).Observe(time.Since(start).Seconds())
	}
}

func (metrics *Metrics) Handler(token string) http.Handler {
	scrapeHandler := promhttp.HandlerFor(metrics.registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			http.Error(w, "metrics authentication is not configured", http.StatusServiceUnavailable)
			return
		}
		authorization := r.Header.Get("Authorization")
		if !strings.HasPrefix(authorization, "Bearer ") {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		provided := strings.TrimPrefix(authorization, "Bearer ")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		scrapeHandler.ServeHTTP(w, r)
	})
}
