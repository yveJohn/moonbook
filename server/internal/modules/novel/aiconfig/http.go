package aiconfig

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
	handler := &Handler{service: service}
	read := private.Group("novel/aiConfigs")
	write := private.Group("novel/aiConfigs").Use(middleware.OperationRecord())
	read.GET("", handler.list)
	read.GET("/enabledOptions", handler.enabledOptions)
	read.GET("/:id", handler.get)
	write.POST("", handler.create)
	write.PUT("/:id", handler.update)
	write.POST("/:id/resetModelState", handler.resetModelState)
	write.DELETE("/:id", handler.delete)
}

func bindInput(ctx *gin.Context) (Input, error) {
	var input Input
	if err := ctx.ShouldBindJSON(&input); err != nil {
		return Input{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "AI 配置参数无效")
	}
	return input, nil
}

func (handler *Handler) list(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "20"))
	result, err := handler.service.List(ctx, ctx.Query("keyword"), ctx.Query("enabled"), page, pageSize)
	if err != nil {
		apperror.WriteManagement(ctx, err)
		return
	}
	managementresponse.OK(ctx, result, "获取成功")
}

func (handler *Handler) enabledOptions(ctx *gin.Context) {
	result, err := handler.service.EnabledOptions(ctx)
	if err != nil {
		apperror.WriteManagement(ctx, err)
		return
	}
	managementresponse.OK(ctx, result, "获取成功")
}

func (handler *Handler) get(ctx *gin.Context) {
	result, err := handler.service.Get(ctx, ctx.Param("id"))
	if err != nil {
		apperror.WriteManagement(ctx, err)
		return
	}
	managementresponse.OK(ctx, result, "获取成功")
}

func (handler *Handler) create(ctx *gin.Context) {
	input, err := bindInput(ctx)
	if err == nil {
		var result Config
		result, err = handler.service.Create(ctx, input)
		if err == nil {
			managementresponse.OK(ctx, result, "创建成功")
			return
		}
	}
	apperror.WriteManagement(ctx, err)
}

func (handler *Handler) update(ctx *gin.Context) {
	input, err := bindInput(ctx)
	if err == nil {
		var result Config
		result, err = handler.service.Update(ctx, ctx.Param("id"), input)
		if err == nil {
			managementresponse.OK(ctx, result, "更新成功")
			return
		}
	}
	apperror.WriteManagement(ctx, err)
}

func (handler *Handler) resetModelState(ctx *gin.Context) {
	result, err := handler.service.ResetModelState(ctx, ctx.Param("id"))
	if err != nil {
		apperror.WriteManagement(ctx, err)
		return
	}
	managementresponse.OK(ctx, result, "模型状态已重置")
}

func (handler *Handler) delete(ctx *gin.Context) {
	if err := handler.service.Delete(ctx, ctx.Param("id")); err != nil {
		apperror.WriteManagement(ctx, err)
		return
	}
	managementresponse.OK(ctx, nil, "删除成功")
}
