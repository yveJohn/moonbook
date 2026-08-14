package chaptersummary

import (
	"net/http"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func RegisterRoutes(group *gin.RouterGroup, service *Service) {
	handler := &Handler{service: service}
	read := group.Group("novel/chapterSummary")
	write := group.Group("novel/chapterSummary").Use(middleware.OperationRecord())
	read.GET("/config", handler.config)
	read.GET("/status", handler.status)
	read.GET("/tasks", handler.tasks)
	read.GET("/tasks/:id", handler.task)
	write.PUT("/config", handler.saveConfig)
	write.POST("/tasks", handler.start)
	write.POST("/tasks/:id/stop", handler.stop)
	write.POST("/tasks/:id/resume", handler.resume)
}

func bindJSON[T any](ctx *gin.Context) (T, error) {
	var value T
	if ctx.ShouldBindJSON(&value) != nil {
		return value, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效")
	}
	return value, nil
}

func respond(ctx *gin.Context, value any, err error, message string) {
	if err != nil {
		apperror.WriteManagement(ctx, err)
		return
	}
	managementresponse.OK(ctx, value, message)
}

func (handler *Handler) config(ctx *gin.Context) {
	value, err := handler.service.GetConfig(ctx)
	respond(ctx, value, err, "获取成功")
}

func (handler *Handler) saveConfig(ctx *gin.Context) {
	input, err := bindJSON[ConfigInput](ctx)
	if err != nil {
		apperror.WriteManagement(ctx, err)
		return
	}
	value, err := handler.service.SaveConfig(ctx, input)
	respond(ctx, value, err, "保存成功")
}

func (handler *Handler) status(ctx *gin.Context) {
	value, err := handler.service.GetStatus(ctx)
	respond(ctx, value, err, "获取成功")
}

func (handler *Handler) tasks(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "20"))
	items, total, err := handler.service.ListTasks(ctx, ctx.Query("status"), page, size)
	if err != nil {
		apperror.WriteManagement(ctx, err)
		return
	}
	managementresponse.OK(ctx, managementresponse.Page{List: items, Total: total, Page: page, PageSize: size}, "获取成功")
}

func (handler *Handler) task(ctx *gin.Context) {
	id, err := parseID(ctx.Param("id"), "任务 ID")
	var value Task
	if err == nil {
		value, err = handler.service.GetTask(ctx, id, true)
	}
	respond(ctx, value, err, "获取成功")
}

func (handler *Handler) start(ctx *gin.Context) {
	input, err := bindJSON[StartInput](ctx)
	if err != nil {
		apperror.WriteManagement(ctx, err)
		return
	}
	value, err := handler.service.Start(ctx, input)
	respond(ctx, value, err, "任务已启动")
}

func (handler *Handler) stop(ctx *gin.Context) {
	id, err := parseID(ctx.Param("id"), "任务 ID")
	if err == nil {
		err = handler.service.Stop(ctx, id)
	}
	respond(ctx, nil, err, "已请求停止")
}

func (handler *Handler) resume(ctx *gin.Context) {
	id, err := parseID(ctx.Param("id"), "任务 ID")
	var value Task
	if err == nil {
		value, err = handler.service.Resume(ctx, id)
	}
	respond(ctx, value, err, "任务已续跑")
}
