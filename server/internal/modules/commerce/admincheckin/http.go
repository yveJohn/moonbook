package admincheckin

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
	read.GET("checkinRules", h.list)
	write.POST("checkinRules", h.create)
	write.PUT("checkinRules/:id", h.update)
	write.DELETE("checkinRules/:id", h.delete)
}
func render(v Rule) map[string]any {
	return map[string]any{"id": v.ID, "ruleType": v.RuleType, "continuousDays": v.ContinuousDays, "rewardMode": v.RewardMode, "fixedCoin": v.FixedCoin, "minCoin": v.MinCoin, "maxCoin": v.MaxCoin, "status": v.Status, "sortOrder": v.SortOrder, "remark": v.Remark}
}
func (h *Handler) list(c *gin.Context) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	n, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	rows, total, err := h.service.List(c, c.Query("ruleType"), p, n)
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
	var v Input
	if err := c.ShouldBindJSON(&v); err != nil {
		return v, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效")
	}
	return v, nil
}
func (h *Handler) create(c *gin.Context) {
	v, err := input(c)
	if err == nil {
		var r Rule
		r, err = h.service.Create(c, v)
		if err == nil {
			managementresponse.OK(c, render(r), "创建成功")
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
	v, e := input(c)
	if e == nil {
		var r Rule
		r, e = h.service.Update(c, id, v)
		if e == nil {
			managementresponse.OK(c, render(r), "更新成功")
			return
		}
	}
	apperror.WriteManagement(c, e)
}
func (h *Handler) delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err == nil && id > 0 {
		err = h.service.Delete(c, id)
	} else if err == nil {
		err = apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串")
	}
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, nil, "删除成功")
}
