package adminuser

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
	read.GET("users", h.list)
	read.GET("users/:id", h.get)
	read.GET("users/:id/operations", h.operations)
	write.PUT("users/:id/status", h.status)
	write.PUT("users/:id/password", h.password)
}
func (h *Handler) password(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串"))
		return
	}
	var req struct {
		Password        string `json:"password"`
		ConfirmPassword string `json:"confirmPassword"`
	}
	if c.ShouldBindJSON(&req) != nil {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效"))
		return
	}
	if err := h.service.ResetPassword(c, id, req.Password, req.ConfirmPassword); err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, nil, "密码已重置")
}
func render(u User) map[string]any {
	recharge, bonus := u.RechargeBalance, u.BonusBalance
	if recharge == "" {
		recharge = "0"
	}
	if bonus == "" {
		bonus = "0"
	}
	return map[string]any{"id": u.ID, "username": u.Username, "nickname": u.Nickname, "status": u.Status, "passwordAlgorithm": u.PasswordAlgorithm, "lastLoginAt": u.LastLoginAt, "createdAt": u.CreatedAt, "updatedAt": u.UpdatedAt, "rechargeBalance": recharge, "bonusBalance": bonus}
}
func (h *Handler) list(c *gin.Context) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	n, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	rows, total, err := h.service.List(c, c.Query("keyword"), c.Query("status"), p, n)
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
func (h *Handler) get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串"))
		return
	}
	u, err := h.service.Get(c, id)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, render(u), "获取成功")
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
	u, err := h.service.SetStatus(c, id, req.Status)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, render(u), "更新成功")
}

func (h *Handler) operations(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串"))
		return
	}
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	n, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	rows, total, err := h.service.ListOperations(c, id, p, n)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	items := make([]any, 0, len(rows))
	for _, v := range rows {
		items = append(items, renderOperation(v))
	}
	managementresponse.OK(c, managementresponse.Page{List: items, Total: total, Page: p, PageSize: n}, "获取成功")
}
func renderOperation(v Operation) map[string]any {
	return map[string]any{
		"id":           v.ID,
		"createdAt":    v.CreatedAt,
		"operatorId":   v.OperatorID,
		"operatorName": v.OperatorName,
		"action":       v.Action,
		"method":       v.Method,
		"path":         v.Path,
		"status":       v.Status,
		"summary":      v.Summary,
		"errorMessage": v.ErrorMessage,
	}
}
