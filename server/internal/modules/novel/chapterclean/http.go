package chapterclean

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
	h := &Handler{service}
	read := group.Group("novel/chapterClean")
	write := group.Group("novel/chapterClean").Use(middleware.OperationRecord())
	read.GET("/config", h.config)
	read.GET("/tasks", h.tasks)
	read.GET("/tasks/:id", h.task)
	read.GET("/results", h.results)
	read.GET("/results/:id", h.result)
	write.PUT("/config", h.save)
	write.POST("/tasks", h.start)
	write.POST("/tasks/:id/stop", h.stop)
	write.POST("/tasks/:id/resume", h.resume)
	write.PUT("/results/:id/review", h.review)
	write.POST("/results/:id/reclean", h.reclean)
}
func bind[T any](c *gin.Context) (T, error) {
	var v T
	if c.ShouldBindJSON(&v) != nil {
		return v, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效")
	}
	return v, nil
}
func pathID(c *gin.Context) (int64, error) { return parseLong(c.Param("id"), "ID") }
func (h *Handler) config(c *gin.Context) {
	v, e := h.service.GetConfig(c)
	respond(c, v, e, "获取成功")
}
func (h *Handler) save(c *gin.Context) {
	v, e := bind[ConfigInput](c)
	if e == nil {
		var out Config
		out, e = h.service.SaveConfig(c, v)
		respond(c, out, e, "保存成功")
		return
	}
	apperror.WriteManagement(c, e)
}
func page(c *gin.Context) (int, int) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	s, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	return p, s
}
func (h *Handler) tasks(c *gin.Context) {
	p, s := page(c)
	v, total, e := h.service.ListTasks(c, c.Query("status"), c.Query("bookId"), p, s)
	if e != nil {
		apperror.WriteManagement(c, e)
		return
	}
	managementresponse.OK(c, managementresponse.Page{List: v, Total: total, Page: p, PageSize: s}, "获取成功")
}
func (h *Handler) task(c *gin.Context) {
	id, e := pathID(c)
	if e == nil {
		var v Task
		v, e = h.service.GetTask(c, id)
		respond(c, v, e, "获取成功")
		return
	}
	apperror.WriteManagement(c, e)
}
func (h *Handler) start(c *gin.Context) {
	v, e := bind[StartInput](c)
	if e == nil {
		var out Task
		out, e = h.service.Start(c, v)
		respond(c, out, e, "任务已启动")
		return
	}
	apperror.WriteManagement(c, e)
}
func (h *Handler) stop(c *gin.Context) {
	id, e := pathID(c)
	if e == nil {
		e = h.service.Stop(c, id)
	}
	respond(c, nil, e, "已请求停止")
}
func (h *Handler) resume(c *gin.Context) {
	id, e := pathID(c)
	if e == nil {
		var v Task
		v, e = h.service.Resume(c, id)
		respond(c, v, e, "任务已续跑")
		return
	}
	apperror.WriteManagement(c, e)
}
func (h *Handler) results(c *gin.Context) {
	p, s := page(c)
	v, total, e := h.service.ListResults(c, c.Query("status"), p, s)
	if e != nil {
		apperror.WriteManagement(c, e)
		return
	}
	managementresponse.OK(c, managementresponse.Page{List: v, Total: total, Page: p, PageSize: s}, "获取成功")
}
func (h *Handler) result(c *gin.Context) {
	id, e := pathID(c)
	if e == nil {
		var v Result
		v, e = h.service.GetResult(c, id, true)
		respond(c, v, e, "获取成功")
		return
	}
	apperror.WriteManagement(c, e)
}
func (h *Handler) review(c *gin.Context) {
	id, e := pathID(c)
	var v ReviewInput
	if e == nil {
		v, e = bind[ReviewInput](c)
	}
	if e == nil {
		e = h.service.Review(c, id, v)
	}
	respond(c, nil, e, "审核成功")
}
func (h *Handler) reclean(c *gin.Context) {
	id, e := pathID(c)
	if e == nil {
		e = h.service.Reclean(c, id)
	}
	respond(c, nil, e, "已重置为待清洗")
}
func respond(c *gin.Context, v any, e error, message string) {
	if e != nil {
		apperror.WriteManagement(c, e)
		return
	}
	managementresponse.OK(c, v, message)
}
