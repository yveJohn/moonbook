package apperror

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

func TestWriteManagementUsesGVAEnvelopeAndRedactsCause(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestMeta())
	router.GET("/failure", func(c *gin.Context) {
		WriteManagement(c, Wrap(
			errors.New("postgres://user:secret@internal/database"),
			CodeUnavailable,
			http.StatusServiceUnavailable,
			"service temporarily unavailable",
		))
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/failure", nil)
	request.Header.Set("X-Request-Id", "request-123")
	router.ServeHTTP(recorder, request)

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	for _, want := range []string{`"code":7`, `"errorCode":"UNAVAILABLE"`, `"requestId":"request-123"`, `"msg":"service temporarily unavailable"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %s: %s", want, body)
		}
	}
	if strings.Contains(body, "secret") || strings.Contains(body, "postgres://") {
		t.Fatalf("response leaked cause: %s", body)
	}
	if len(recorder.Result().Header.Get("X-Trace-Id")) != 32 {
		t.Fatalf("missing trace ID: %q", recorder.Result().Header.Get("X-Trace-Id"))
	}
}
