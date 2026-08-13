package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHTTPMetricsUseRouteTemplatesAndExposeStandardMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	metrics := New(nil)
	router := gin.New()
	router.Use(metrics.Middleware())
	router.GET("/books/:id", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/books/9223372036854775807", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}

	metricsRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	request.Header.Set("Authorization", "Bearer test-token")
	metrics.Handler("test-token").ServeHTTP(metricsRecorder, request)
	body := metricsRecorder.Body.String()
	for _, want := range []string{
		`moonbook_http_requests_total{method="GET",route="/books/:id",status="204"} 1`,
		`moonbook_http_request_duration_seconds_count{method="GET",route="/books/:id"} 1`,
		"go_goroutines ",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics missing %q", want)
		}
	}
	if strings.Contains(body, "9223372036854775807") {
		t.Fatal("metrics contain high-cardinality path parameter")
	}
}

func TestMetricsHandlerRequiresConfiguredBearerToken(t *testing.T) {
	metrics := New(nil)
	for _, tt := range []struct {
		name       string
		token      string
		authority  string
		wantStatus int
	}{
		{name: "missing configuration", wantStatus: http.StatusServiceUnavailable},
		{name: "missing authorization", token: "secret", wantStatus: http.StatusUnauthorized},
		{name: "bare token", token: "secret", authority: "secret", wantStatus: http.StatusUnauthorized},
		{name: "wrong authorization", token: "secret", authority: "Bearer other", wantStatus: http.StatusUnauthorized},
		{name: "valid authorization", token: "secret", authority: "Bearer secret", wantStatus: http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			request.Header.Set("Authorization", tt.authority)
			metrics.Handler(tt.token).ServeHTTP(recorder, request)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
		})
	}
}
