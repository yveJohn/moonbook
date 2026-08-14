package fetchlog

import (
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type Handler struct{ service *Service }

func RegisterRoutes(private *gin.RouterGroup, service *Service) {
	h := &Handler{service: service}
	g := private.Group("novel/crawl")
	g.GET("fetchLogs", h.list)
	g.GET("fetchLogs/:id", h.get)
}
func render(v Log) map[string]any {
	return map[string]any{"id": v.ID, "taskId": v.TaskID, "sourceId": v.SourceID, "sourceName": v.SourceName, "boardId": v.BoardID, "boardName": v.BoardName, "threadUrl": v.ThreadURL, "requestUrl": v.RequestURL, "stage": v.Stage, "status": v.Status, "httpStatus": v.HTTPStatus, "responseBytes": v.ResponseBytes, "elapsedMs": v.ElapsedMs, "itemCount": v.ItemCount, "targetBookId": v.TargetBookID, "message": v.Message, "detailJson": v.DetailJSON, "createdAt": v.CreatedAt}
}
func (h *Handler) list(c *gin.Context) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	n, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	rows, total, err := h.service.List(c, c.Query("keyword"), c.Query("status"), c.Query("stage"), c.Query("taskId"), p, n)
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
		err = apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串")
	}
	if err == nil {
		v, e := h.service.Get(c, id)
		if e == nil {
			managementresponse.OK(c, render(v), "获取成功")
			return
		}
		err = e
	}
	apperror.WriteManagement(c, err)
}
