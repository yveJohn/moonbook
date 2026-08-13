package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/config"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/gin-gonic/gin"
)

func TestCorsByRulesStrictWhitelist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := global.GVA_CONFIG.Cors
	t.Cleanup(func() { global.GVA_CONFIG.Cors = old })
	global.GVA_CONFIG.Cors = config.CORS{
		Mode: "strict-whitelist",
		Whitelist: []config.CORSWhitelist{{
			AllowOrigin:      "http://localhost:8080",
			AllowMethods:     "GET,OPTIONS",
			AllowHeaders:     "Content-Type",
			ExposeHeaders:    "Content-Length",
			AllowCredentials: true,
		}},
	}

	router := gin.New()
	router.Use(CorsByRules())
	router.GET("/health/live", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.OPTIONS("/health/live", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	tests := []struct {
		name            string
		method          string
		origin          string
		wantStatus      int
		wantAllowOrigin string
	}{
		{name: "non browser request", method: http.MethodGet, wantStatus: http.StatusOK},
		{name: "allowed origin", method: http.MethodGet, origin: "http://localhost:8080", wantStatus: http.StatusOK, wantAllowOrigin: "http://localhost:8080"},
		{name: "rejected origin", method: http.MethodGet, origin: "https://untrusted.example", wantStatus: http.StatusForbidden},
		{name: "allowed preflight", method: http.MethodOptions, origin: "http://localhost:8080", wantStatus: http.StatusNoContent, wantAllowOrigin: "http://localhost:8080"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, "/health/live", nil)
			if tt.origin != "" {
				request.Header.Set("Origin", tt.origin)
			}
			router.ServeHTTP(recorder, request)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status=%d, want=%d", recorder.Code, tt.wantStatus)
			}
			if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != tt.wantAllowOrigin {
				t.Fatalf("allow origin=%q, want=%q", got, tt.wantAllowOrigin)
			}
		})
	}
}
