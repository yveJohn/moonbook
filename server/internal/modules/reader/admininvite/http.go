package admininvite

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
	read := private.Group("reader")
	write := private.Group("reader").Use(middleware.OperationRecord())
	read.GET("inviteCodes", h.list)
	write.POST("inviteCodes", h.create)
	write.PUT("inviteCodes/:id", h.update)
	write.PUT("inviteCodes/:id/status", h.status)
	write.DELETE("inviteCodes/:id", h.delete)
}
func render(v InviteCode) map[string]any {
	return map[string]any{"id": v.ID, "inviterReaderId": v.InviterReaderID, "code": v.Code, "status": v.Status, "maxUseCount": v.MaxUseCount, "usedCount": v.UsedCount, "expiresAt": v.ExpiresAt, "remark": v.Remark, "createdAt": v.CreatedAt}
}
func (h *Handler) update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串"))
		return
	}
	var req UpdateInput
	if c.ShouldBindJSON(&req) != nil {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效"))
		return
	}
	v, err := h.service.Update(c, id, req)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, render(v), "更新成功")
}
func (h *Handler) list(c *gin.Context) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	n, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	rows, total, err := h.service.List(c, c.Query("keyword"), p, n)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	items := make([]any, 0, len(rows))
	for _, v := range rows {
		items = append(items, render(v))
	}
	managementresponse.OK(c, managementresponse.Page{List: items, Total: total, Page: p, PageSize: n}, "获取成功")
}
func (h *Handler) create(c *gin.Context) {
	var req CreateInput
	if c.ShouldBindJSON(&req) != nil {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效"))
		return
	}
	v, err := h.service.Create(c, req)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, render(v), "创建成功")
}
func (h *Handler) status(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串"))
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if c.ShouldBindJSON(&req) != nil {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效"))
		return
	}
	v, err := h.service.SetStatus(c, id, req.Status)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, render(v), "更新成功")
}
func (h *Handler) delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串"))
		return
	}
	if err = h.service.Delete(c, id); err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, nil, "删除成功")
}
