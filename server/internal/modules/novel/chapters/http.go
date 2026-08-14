package chapters

import (
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

type request struct {
	BookID        string  `json:"bookId" binding:"required"`
	ChapterNo     *int    `json:"chapterNo"`
	ChapterName   string  `json:"chapterName" binding:"required"`
	IsVIP         *bool   `json:"isVip" binding:"required"`
	BookPriceCoin string  `json:"bookPriceCoin" binding:"required"`
	ChapterStatus string  `json:"chapterStatus" binding:"required"`
	AICleanStatus string  `json:"aiCleanStatus" binding:"required"`
	Content       *string `json:"content"`
}

type response struct {
	ID            string `json:"id"`
	BookID        string `json:"bookId"`
	ChapterNo     int    `json:"chapterNo"`
	ChapterName   string `json:"chapterName"`
	WordCount     int    `json:"wordCount"`
	IsVIP         bool   `json:"isVip"`
	BookPriceCoin string `json:"bookPriceCoin"`
	ChapterStatus string `json:"chapterStatus"`
	AICleanStatus string `json:"aiCleanStatus"`
	SourceType    string `json:"sourceType"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

type contentResponse struct {
	response
	Content string `json:"content"`
	Version int    `json:"version"`
	SHA256  string `json:"sha256"`
	Bytes   int64  `json:"bytes"`
}

func RegisterRoutes(group *gin.RouterGroup, db *sql.DB, objects *objectstore.Service) {
	handler := &Handler{service: NewService(db, objects)}
	read := group.Group("novel")
	write := group.Group("novel").Use(middleware.OperationRecord())
	read.GET("chapters", handler.list)
	read.GET("chapters/:id", handler.get)
	read.GET("chapters/:id/content", handler.content)
	write.POST("chapters", handler.create)
	write.PUT("chapters/:id", handler.update)
	write.DELETE("chapters/:id", handler.delete)
}

func parsePositiveLong(raw, name string) (int64, error) {
	if raw == "" || strings.TrimSpace(raw) != raw || raw[0] == '+' || raw[0] == '-' {
		return 0, invalid(name + "必须是正整数字符串")
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, invalid(name + "必须是正整数字符串")
	}
	return value, nil
}

func parseNonNegativeLong(raw, name string) (int64, error) {
	if raw == "" || strings.TrimSpace(raw) != raw || raw[0] == '+' || raw[0] == '-' {
		return 0, invalid(name + "必须是非负整数字符串")
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, invalid(name + "必须是非负整数字符串")
	}
	return value, nil
}

func parseRequest(body request) (Input, error) {
	bookID, err := parsePositiveLong(body.BookID, "书籍 ID")
	if err != nil {
		return Input{}, err
	}
	price, err := parseNonNegativeLong(body.BookPriceCoin, "章节价格")
	if err != nil {
		return Input{}, err
	}
	if body.IsVIP == nil {
		return Input{}, invalid("是否收费不能为空")
	}
	return Input{BookID: bookID, ChapterNo: body.ChapterNo, ChapterName: body.ChapterName, IsVIP: *body.IsVIP, BookPriceCoin: price, ChapterStatus: body.ChapterStatus, AICleanStatus: body.AICleanStatus, Content: body.Content}, nil
}

func toResponse(chapter Chapter) response {
	return response{ID: strconv.FormatInt(chapter.ID, 10), BookID: strconv.FormatInt(chapter.BookID, 10), ChapterNo: chapter.ChapterNo, ChapterName: chapter.ChapterName, WordCount: chapter.WordCount, IsVIP: chapter.IsVIP, BookPriceCoin: strconv.FormatInt(chapter.BookPriceCoin, 10), ChapterStatus: chapter.ChapterStatus, AICleanStatus: chapter.AICleanStatus, SourceType: chapter.SourceType, CreatedAt: chapter.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: chapter.UpdatedAt.UTC().Format(time.RFC3339Nano)}
}

func parseID(c *gin.Context) (int64, error) { return parsePositiveLong(c.Param("id"), "章节 ID") }

func (handler *Handler) list(c *gin.Context) {
	page, err := parsePage(c.Query("page"), 1)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	pageSize, err := parsePage(c.Query("pageSize"), 10)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	var bookID int64
	if raw := c.Query("bookId"); raw != "" {
		bookID, err = parsePositiveLong(raw, "书籍 ID")
		if err != nil {
			apperror.WriteManagement(c, err)
			return
		}
	}
	result, err := handler.service.List(c.Request.Context(), Filter{Page: page, PageSize: pageSize, BookID: bookID, Keyword: c.Query("keyword"), ChapterStatus: c.Query("chapterStatus"), AICleanStatus: c.Query("aiCleanStatus")})
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	items := make([]response, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toResponse(item))
	}
	managementresponse.OK(c, managementresponse.Page{List: items, Total: result.Total, Page: result.Page, PageSize: result.PageSize}, "获取成功")
}

func parsePage(raw string, fallback int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, invalid("分页参数无效")
	}
	return value, nil
}

func (handler *Handler) get(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	chapter, err := handler.service.Get(c.Request.Context(), id)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, toResponse(chapter), "获取成功")
}

func (handler *Handler) content(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	content, err := handler.service.ReadContent(c.Request.Context(), id)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, contentResponse{response: toResponse(content.Chapter), Content: content.Text, Version: content.Version, SHA256: content.SHA256, Bytes: content.Bytes}, "获取成功")
}

func bind(c *gin.Context) (Input, error) {
	var body request
	if err := c.ShouldBindJSON(&body); err != nil {
		return Input{}, invalid("请求参数无效")
	}
	return parseRequest(body)
}

func (handler *Handler) create(c *gin.Context) {
	input, err := bind(c)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	chapter, err := handler.service.Create(c.Request.Context(), input)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, toResponse(chapter), "创建成功")
}

func (handler *Handler) update(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	input, err := bind(c)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	chapter, err := handler.service.Update(c.Request.Context(), id, input)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, toResponse(chapter), "更新成功")
}

func (handler *Handler) delete(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	if err := handler.service.Delete(c.Request.Context(), id); err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.Message(c, "删除成功")
}
