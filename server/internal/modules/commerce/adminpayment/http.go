package adminpayment

import (
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type Handler struct{ service *Service }

func RegisterRoutes(private *gin.RouterGroup, service *Service) {
	h := &Handler{service: service}
	read := private.Group("reader/payment")
	write := private.Group("reader/payment").Use(middleware.OperationRecord())
	read.GET("channels", h.list)
	write.PUT("channels/:id", h.update)
}
func render(v Channel) map[string]any {
	return map[string]any{"id": strconv.FormatInt(v.ID, 10), "provider": v.Provider, "enabled": v.Enabled, "currency": v.Currency, "token": v.Token, "network": v.Network, "configured": v.Configured}
}
func (h *Handler) list(c *gin.Context) {
	rows, err := h.service.List(c)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	out := make([]any, 0, len(rows))
	for _, v := range rows {
		out = append(out, render(v))
	}
	managementresponse.OK(c, out, "获取成功")
}
func (h *Handler) update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串"))
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if c.ShouldBindJSON(&req) != nil || req.Enabled == nil {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "enabled参数无效"))
		return
	}
	v, err := h.service.SetEnabled(c, id, *req.Enabled)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, render(v), "更新成功")
}
