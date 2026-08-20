package adminrechargeorder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/gin-gonic/gin"
)

func TestCallbackFilterValidationRejectsInvalidContracts(t *testing.T) {
	now := time.Now().UTC()
	before := now.Add(-time.Hour)
	tooEarly := now.Add(-32 * 24 * time.Hour)
	badStatus := 700
	tests := []CallbackFilter{
		{SignatureStatus: "false"},
		{ProcessingResult: "SUCCESS"},
		{FailureCode: "lowercase"},
		{ResponseStatus: &badStatus},
		{StartTime: &now, EndTime: &before},
		{StartTime: &tooEarly, EndTime: &now},
	}
	service := NewService(SQLRepository{}, nil, nil)
	for _, filter := range tests {
		if _, _, err := service.ListCallbacks(context.Background(), filter); err == nil || apperror.Expose(err).HTTPStatus != http.StatusBadRequest {
			t.Fatalf("filter=%+v err=%v", filter, err)
		}
	}
}

func TestCallbackFilterQueryRequiresExplicitTimezone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, values := range []url.Values{
		{"startTime": {"2026-08-20T10:00:00"}},
		{"startTime": {"2026-08-20T10:00:00Z"}, "endTime": {"2026-07-01T10:00:00Z"}},
		{"responseStatus": {"not-a-number"}},
	} {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest(http.MethodGet, "/?"+values.Encode(), nil)
		if _, err := callbackFilterFromQuery(context); err == nil || apperror.Expose(err).HTTPStatus != http.StatusBadRequest {
			t.Fatalf("values=%v err=%v", values, err)
		}
	}
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodGet, "/?startTime=2026-08-20T10:00:00%2B08:00&endTime=2026-08-21T10:00:00%2B08:00", nil)
	filter, err := callbackFilterFromQuery(context)
	if err != nil || filter.StartTime == nil || filter.EndTime == nil {
		t.Fatalf("filter=%+v err=%v", filter, err)
	}
}
