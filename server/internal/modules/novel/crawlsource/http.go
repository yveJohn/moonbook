package crawlsource

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
	read := private.Group("novel/crawl")
	write := private.Group("novel/crawl").Use(middleware.OperationRecord())
	read.GET("sources", h.list)
	write.POST("sources", h.create)
	write.PUT("sources/:id", h.update)
	write.DELETE("sources/:id", h.delete)
}

func render(v Source) map[string]any {
	return map[string]any{"id": v.ID, "sourceName": v.SourceName, "baseUrl": v.BaseURL, "requestCharset": v.RequestCharset, "cookieConfigured": v.CookieConfigured, "userAgent": v.UserAgent, "requestIntervalMs": v.RequestIntervalMs, "enabled": v.Enabled, "sortOrder": v.SortOrder, "remark": v.Remark, "createdAt": v.CreatedAt, "updatedAt": v.UpdatedAt}
}
func (h *Handler) list(c *gin.Context) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	n, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	rows, total, err := h.service.List(c, c.Query("keyword"), c.Query("enabled"), p, n)
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
func input(c *gin.Context) (Input, error) {
	var in Input
	if err := c.ShouldBindJSON(&in); err != nil {
		return in, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效")
	}
	return in, nil
}
func (h *Handler) create(c *gin.Context) {
	in, err := input(c)
	if err == nil {
		var v Source
		v, err = h.service.Create(c, in)
		if err == nil {
			managementresponse.OK(c, render(v), "创建成功")
			return
		}
	}
	apperror.WriteManagement(c, err)
}
func (h *Handler) update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串"))
		return
	}
	in, e := input(c)
	if e == nil {
		var v Source
		v, e = h.service.Update(c, id, in)
		if e == nil {
			managementresponse.OK(c, render(v), "更新成功")
			return
		}
	}
	apperror.WriteManagement(c, e)
}
func (h *Handler) delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err == nil && id > 0 {
		err = h.service.Delete(c, id)
	} else {
		err = apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串")
	}
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, nil, "删除成功")
}
