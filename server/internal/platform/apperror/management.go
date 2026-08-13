package apperror

import (
	"net/http"

	"github.com/flipped-aurora/gin-vue-admin/server/utils/logger"
	"github.com/gin-gonic/gin"
)

type managementResponse struct {
	Code int            `json:"code"`
	Data managementData `json:"data"`
	Msg  string         `json:"msg"`
}

type managementData struct {
	ErrorCode Code   `json:"errorCode"`
	RequestID string `json:"requestId,omitempty"`
	TraceID   string `json:"traceId,omitempty"`
}

func WriteManagement(c *gin.Context, err error) {
	public := Expose(err)
	fields := logger.FromCtx(c.Request.Context())
	c.JSON(http.StatusOK, managementResponse{
		Code: 7,
		Data: managementData{
			ErrorCode: public.Code,
			RequestID: fields.GetRequestID(),
			TraceID:   fields.GetTraceID(),
		},
		Msg: public.Message,
	})
}
