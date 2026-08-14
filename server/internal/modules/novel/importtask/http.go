package importtask

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
	read := private.Group("novel/crawl")
	write := private.Group("novel/crawl").Use(middleware.OperationRecord())
	read.GET("importTasks", h.list)
	read.GET("importTasks/:id", h.get)
	write.POST("importTasks", h.create)
	write.POST("importTasks/:id/retry", h.retry)
	write.POST("importTasks/:id/cancel", h.cancel)
}
func render(v Task) map[string]any {
	return map[string]any{"id": v.ID, "candidateId": v.CandidateID, "sourceId": v.SourceID, "sourceName": v.SourceName, "boardId": v.BoardID, "boardName": v.BoardName, "forumThreadId": v.ForumThreadID, "threadTitle": v.ThreadTitle, "displayTitle": v.DisplayTitle, "threadUrl": v.ThreadURL, "targetBookId": v.TargetBookID, "importMode": v.ImportMode, "mergeStrategy": v.MergeStrategy, "status": v.Status, "qualityStatus": v.QualityStatus, "totalChapterCount": v.TotalChapterCount, "importedChapterCount": v.ImportedChapterCount, "emptyChapterCount": v.EmptyChapterCount, "duplicateChapterCount": v.DuplicateChapterCount, "attemptCount": v.AttemptCount, "maxAttempts": v.MaxAttempts, "qualitySummary": v.QualitySummary, "failReason": v.FailReason, "operatorName": v.OperatorName, "startTime": v.StartTime, "endTime": v.EndTime, "createdAt": v.CreatedAt, "updatedAt": v.UpdatedAt}
}
func (h *Handler) list(c *gin.Context) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	n, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	rows, total, err := h.service.List(c, c.Query("keyword"), c.Query("status"), c.Query("sourceId"), c.Query("boardId"), p, n)
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
func parseID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串")
	}
	return id, nil
}
func decode(c *gin.Context) (CreateInput, error) {
	var in CreateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		return in, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效")
	}
	return in, nil
}
func (h *Handler) get(c *gin.Context) {
	id, err := parseID(c.Param("id"))
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
func (h *Handler) create(c *gin.Context) {
	in, err := decode(c)
	if err == nil {
		v, e := h.service.Create(c, in)
		if e == nil {
			managementresponse.OK(c, render(v), "创建成功")
			return
		}
		err = e
	}
	apperror.WriteManagement(c, err)
}
func (h *Handler) retry(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	var in CreateInput
	if err == nil {
		in, err = decode(c)
	}
	if err == nil {
		v, e := h.service.Retry(c, id, in)
		if e == nil {
			managementresponse.OK(c, render(v), "已重新排队")
			return
		}
		err = e
	}
	apperror.WriteManagement(c, err)
}
func (h *Handler) cancel(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err == nil {
		err = h.service.Cancel(c, id)
	}
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, nil, "已取消")
}
