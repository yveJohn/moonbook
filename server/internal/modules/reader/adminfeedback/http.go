package adminfeedback

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
	read.GET("feedback", h.list)
	read.GET("feedback/:id", h.get)
	write.PUT("feedback/:id/reply", h.reply)
}
func render(v Feedback) map[string]any {
	return map[string]any{"id": v.ID, "readerId": v.ReaderID, "readerUsername": v.ReaderUsername, "bookId": v.BookID, "chapterId": v.ChapterID, "content": v.Content, "status": v.Status, "reply": v.Reply, "createdAt": v.CreatedAt, "repliedAt": v.RepliedAt}
}
func (h *Handler) list(c *gin.Context) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	n, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	rows, total, err := h.service.List(c, c.Query("keyword"), c.Query("status"), p, n)
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
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串"))
		return
	}
	v, err := h.service.Get(c, id)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, render(v), "获取成功")
}
func (h *Handler) reply(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串"))
		return
	}
	var req struct {
		Reply string `json:"reply"`
	}
	if c.ShouldBindJSON(&req) != nil {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效"))
		return
	}
	v, err := h.service.Reply(c, id, req.Reply)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, render(v), "回复成功")
}
