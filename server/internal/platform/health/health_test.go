package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestLive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health/live", Live)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "{\"status\":\"ok\"}" {
		t.Fatalf("unexpected response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestReadyReportsDependencyStatesWithoutErrorDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checker := NewChecker(time.Second, map[string]Probe{
		"postgres": func(context.Context) error { return nil },
		"redis":    func(context.Context) error { return errors.New("secret host redis.internal:6379") },
	})
	router := gin.New()
	router.GET("/health/ready", checker.Ready)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if strings.Contains(body, "redis.internal") || strings.Contains(body, "secret") {
		t.Fatalf("response leaked dependency details: %s", body)
	}
	if body != "{\"status\":\"unavailable\",\"dependencies\":[{\"name\":\"postgres\",\"status\":\"ok\"},{\"name\":\"redis\",\"status\":\"unavailable\"}]}" {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestCheckHonorsTimeout(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	checker := NewChecker(20*time.Millisecond, map[string]Probe{
		"stuck": func(context.Context) error {
			<-release
			return nil
		},
	})
	started := time.Now()
	statuses, err := checker.Check(context.Background())
	if err == nil || len(statuses) != 1 || statuses[0].Name != "stuck" || statuses[0].Status != "unavailable" {
		t.Fatalf("statuses=%v err=%v", statuses, err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("timeout was not enforced")
	}
}
