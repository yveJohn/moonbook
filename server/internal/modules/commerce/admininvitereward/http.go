package admininvitereward

import (
	"net/http"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func RegisterRoutes(private *gin.RouterGroup, service *Service) {
	h := &Handler{service: service}
	read := private.Group("reader")
	write := private.Group("reader").Use(middleware.OperationRecord())
	read.GET("inviteReward", h.get)
	write.PUT("inviteReward", h.update)
}

func render(v Config) map[string]any {
	return map[string]any{"id": v.ID, "enabled": v.Enabled == "true", "inviterRewardCoin": v.InviterRewardCoin, "inviteeRewardCoin": v.InviteeRewardCoin, "remark": v.Remark, "updatedAt": v.UpdatedAt}
}

func (h *Handler) get(c *gin.Context) {
	v, err := h.service.Get(c)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, render(v), "获取成功")
}

func (h *Handler) update(c *gin.Context) {
	var in Input
	if c.ShouldBindJSON(&in) != nil {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效"))
		return
	}
	v, err := h.service.Update(c, in)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, render(v), "更新成功")
}
