package candidate

import (
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
)

type Handler struct{ service *Service }

func RegisterRoutes(private *gin.RouterGroup, service *Service) {
	h := &Handler{service: service}
	read := private.Group("novel/crawl")
	write := private.Group("novel/crawl").Use(middleware.OperationRecord())
	read.GET("candidates", h.list)
	read.GET("candidates/:id", h.get)
	write.PUT("candidates/skip", h.skip)
	write.PUT("candidates/restore", h.restore)
	write.DELETE("candidates", h.delete)
}
func render(v Candidate) map[string]any {
	return map[string]any{"id": v.ID, "sourceId": v.SourceID, "sourceName": v.SourceName, "boardId": v.BoardID, "boardName": v.BoardName, "forumThreadId": v.ForumThreadID, "threadTitle": v.ThreadTitle, "displayTitle": v.DisplayTitle, "threadUrl": v.ThreadURL, "authorId": v.AuthorID, "status": v.Status, "importTaskId": v.ImportTaskID, "targetBookId": v.TargetBookID, "lastImportPageNo": v.LastImportPageNo, "lastImportFloorId": v.LastImportFloorID, "lastFollowTime": v.LastFollowTime, "followCount": v.FollowCount, "followFailReason": v.FollowFailReason, "discoverTime": v.DiscoverTime, "remark": v.Remark, "createdAt": v.CreatedAt, "updatedAt": v.UpdatedAt}
}
func (h *Handler) list(c *gin.Context) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	n, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	rows, total, err := h.service.List(c, c.Query("keyword"), c.Query("sourceId"), c.Query("boardId"), c.Query("status"), p, n)
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

type idsInput struct {
	IDs []string `json:"ids"`
}

func parseID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串")
	}
	return id, nil
}
func decodeIDs(c *gin.Context) ([]int64, error) {
	var in idsInput
	if err := c.ShouldBindJSON(&in); err != nil {
		return nil, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效")
	}
	if len(in.IDs) == 0 {
		return nil, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请选择候选")
	}
	ids := make([]int64, 0, len(in.IDs))
	for _, raw := range in.IDs {
		id, err := parseID(raw)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}
func requestID(c *gin.Context) string { return strings.TrimSpace(c.GetHeader("X-Request-ID")) }
func (h *Handler) skip(c *gin.Context) {
	ids, err := decodeIDs(c)
	if err == nil {
		err = h.service.Skip(c, ids, requestID(c))
	}
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, nil, "已跳过")
}
func (h *Handler) restore(c *gin.Context) {
	ids, err := decodeIDs(c)
	if err == nil {
		err = h.service.Restore(c, ids, requestID(c))
	}
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, nil, "已恢复")
}
func (h *Handler) delete(c *gin.Context) {
	ids, err := decodeIDs(c)
	if err == nil {
		err = h.service.Delete(c, ids)
	}
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, nil, "删除成功")
}
