package bookprofile

import (
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func RegisterRoutes(group *gin.RouterGroup, service *Service) {
	h := &Handler{service}
	read := group.Group("novel/bookProfile")
	write := group.Group("novel/bookProfile").Use(middleware.OperationRecord())
	read.GET("/config", h.config)
	read.GET("/suggestions", h.list)
	read.GET("/suggestions/:id", h.get)
	write.PUT("/config", h.saveConfig)
	write.POST("/suggestions", h.generate)
	write.POST("/suggestions/batch-regenerate", h.batch)
	write.PUT("/suggestions/:id/reject", h.reject)
	write.PUT("/suggestions/:id/apply", h.apply)
}
func bind[T any](c *gin.Context) (T, error) {
	var value T
	if c.ShouldBindJSON(&value) != nil {
		return value, invalid("请求参数无效")
	}
	return value, nil
}
func reply(c *gin.Context, value any, err error, message string) {
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, value, message)
}
func (h *Handler) config(c *gin.Context) {
	v, e := h.service.GetConfig(c)
	reply(c, v, e, "获取成功")
}
func (h *Handler) saveConfig(c *gin.Context) {
	v, e := bind[Config](c)
	if e == nil {
		v, e = h.service.SaveConfig(c, v)
	}
	reply(c, v, e, "保存成功")
}
func (h *Handler) list(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	items, total, e := h.service.List(c, Filter{BookID: c.Query("bookId"), BookName: c.Query("bookName"), Status: c.Query("status"), TriggerType: c.Query("triggerType"), SuggestedCategoryCode: c.Query("suggestedCategoryCode"), Page: page, PageSize: size})
	if e != nil {
		apperror.WriteManagement(c, e)
		return
	}
	managementresponse.OK(c, managementresponse.Page{List: items, Total: total, Page: page, PageSize: size}, "获取成功")
}
func pathID(c *gin.Context) (int64, error) { return parseID(c.Param("id"), "建议 ID") }
func (h *Handler) get(c *gin.Context) {
	id, e := pathID(c)
	var v Suggestion
	if e == nil {
		v, e = h.service.Get(c, id, true)
	}
	reply(c, v, e, "获取成功")
}
func (h *Handler) generate(c *gin.Context) {
	v, e := bind[GenerateInput](c)
	if e == nil {
		trigger := "manual"
		if v.Regenerate {
			trigger = "regenerate"
		}
		v2, err := h.service.Generate(c, v, trigger)
		reply(c, v2, err, "任务已提交")
		return
	}
	reply(c, nil, e, "")
}
func (h *Handler) batch(c *gin.Context) {
	v, e := bind[BatchInput](c)
	var result BatchResult
	if e == nil {
		result, e = h.service.BatchRegenerate(c, v)
	}
	reply(c, result, e, "批量任务已提交")
}
func (h *Handler) reject(c *gin.Context) {
	id, e := pathID(c)
	var v ReviewInput
	if e == nil {
		v, e = bind[ReviewInput](c)
	}
	if e == nil {
		v.ReviewerName = utils.GetUserName(c)
		e = h.service.Reject(c, id, v)
	}
	reply(c, map[string]any{}, e, "已拒绝")
}
func (h *Handler) apply(c *gin.Context) {
	id, e := pathID(c)
	var v ReviewInput
	if e == nil {
		v, e = bind[ReviewInput](c)
	}
	if e == nil {
		v.ReviewerName = utils.GetUserName(c)
		e = h.service.Apply(c, id, v)
	}
	reply(c, map[string]any{}, e, "已应用")
}
