package bookmerge

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
	read := private.Group("novel/bookMerges")
	write := private.Group("novel/bookMerges").Use(middleware.OperationRecord())
	read.GET("", h.list)
	read.GET("/eligibleBooks", h.eligibleBooks)
	read.GET("/:id", h.get)
	write.POST("/preview", h.preview)
	write.POST("/execute", h.execute)
}

func parseID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串")
	}
	return id, nil
}

func (h *Handler) list(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	result, err := h.service.List(c, c.Query("keyword"), c.Query("status"), page, pageSize)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, result, "获取成功")
}

func (h *Handler) eligibleBooks(c *gin.Context) {
	items, err := h.service.EligibleBooks(c, c.Query("keyword"))
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, items, "获取成功")
}

func (h *Handler) get(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err == nil {
		var task Task
		task, err = h.service.Get(c, id)
		if err == nil {
			managementresponse.OK(c, task, "获取成功")
			return
		}
	}
	apperror.WriteManagement(c, err)
}

func (h *Handler) preview(c *gin.Context) {
	var input PreviewInput
	if err := c.ShouldBindJSON(&input); err != nil {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "合并预览参数无效"))
		return
	}
	result, err := h.service.Preview(c, input)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, result, "预览成功")
}

func (h *Handler) execute(c *gin.Context) {
	var input ExecuteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "合并执行参数无效"))
		return
	}
	result, err := h.service.Execute(c, input)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, result, "合并成功")
}
