package adminmembership

import (
	"net/http"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func RegisterRoutes(private *gin.RouterGroup, service *Service) {
	h := &Handler{service: service}
	private.Group("reader").Use(middleware.OperationRecord()).POST("users/:id/membership", h.grant)
}

func render(v Grant) map[string]any {
	return map[string]any{"id": v.ID, "readerId": v.ReaderID, "grantType": v.GrantType, "startsAt": v.StartsAt, "expiresAt": v.ExpiresAt, "permanent": v.Permanent == "true", "status": v.Status, "sourceType": v.SourceType, "sourceRef": v.SourceRef, "remark": v.Remark, "createdAt": v.CreatedAt}
}

func (h *Handler) grant(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串"))
		return
	}
	var in Input
	if c.ShouldBindJSON(&in) != nil {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效"))
		return
	}
	v, err := h.service.Grant(c, id, in)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, render(v), "会员已发放")
}
