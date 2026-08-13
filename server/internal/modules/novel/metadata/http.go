package metadata

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

type categoryRequest struct {
	Code          string  `json:"code" binding:"required"`
	Name          string  `json:"name" binding:"required"`
	Kind          string  `json:"kind" binding:"required"`
	WorkDirection *string `json:"workDirection"`
	Sort          int     `json:"sort"`
	Enabled       *bool   `json:"enabled" binding:"required"`
}

type authorRequest struct {
	PenName       string  `json:"penName" binding:"required"`
	Status        string  `json:"status" binding:"required"`
	WorkDirection *string `json:"workDirection"`
}

type categoryResponse struct {
	ID            string  `json:"id"`
	Code          string  `json:"code"`
	Name          string  `json:"name"`
	Kind          string  `json:"kind"`
	WorkDirection *string `json:"workDirection"`
	Sort          int     `json:"sort"`
	Enabled       bool    `json:"enabled"`
	Source        string  `json:"source"`
	CreatedAt     string  `json:"createdAt"`
	UpdatedAt     string  `json:"updatedAt"`
}

type authorResponse struct {
	ID                 string  `json:"id"`
	PenName            string  `json:"penName"`
	Status             string  `json:"status"`
	WorkDirection      *string `json:"workDirection"`
	Source             string  `json:"source"`
	LegacyAuthorID     *string `json:"legacyAuthorId"`
	LegacyBookAuthorID *string `json:"legacyBookAuthorId"`
	CreatedAt          string  `json:"createdAt"`
	UpdatedAt          string  `json:"updatedAt"`
}

func RegisterRoutes(private *gin.RouterGroup, db *sql.DB) {
	handler := &Handler{service: NewService(db)}
	read := private.Group("novel")
	write := private.Group("novel").Use(middleware.OperationRecord())
	read.GET("categories", handler.listCategories)
	write.POST("categories", handler.createCategory)
	write.PUT("categories/:id", handler.updateCategory)
	write.DELETE("categories/:id", handler.deleteCategory)
	read.GET("authors", handler.listAuthors)
	write.POST("authors", handler.createAuthor)
	write.PUT("authors/:id", handler.updateAuthor)
	write.DELETE("authors/:id", handler.deleteAuthor)
}

func pageFilter(c *gin.Context) (ListFilter, error) {
	page, err := optionalPositiveInt(c.Query("page"), 1)
	if err != nil {
		return ListFilter{}, invalid("page 必须是正整数")
	}
	pageSize, err := optionalPositiveInt(c.Query("pageSize"), 10)
	if err != nil {
		return ListFilter{}, invalid("pageSize 必须是正整数")
	}
	filter := ListFilter{Page: page, PageSize: pageSize, Keyword: c.Query("keyword"), Kind: c.Query("kind"), Status: c.Query("status")}
	if raw, exists := c.GetQuery("enabled"); exists && strings.TrimSpace(raw) != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			return ListFilter{}, invalid("enabled 必须是 true 或 false")
		}
		filter.Enabled = &enabled
	}
	return filter, nil
}

func optionalPositiveInt(raw string, fallback int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, invalid("分页参数无效")
	}
	return value, nil
}

func pathID(c *gin.Context) (int64, error) {
	raw := c.Param("id")
	if raw == "" || strings.TrimSpace(raw) != raw || strings.HasPrefix(raw, "+") {
		return 0, invalid("ID 必须是十进制正整数字符串")
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, invalid("ID 必须是十进制正整数字符串")
	}
	return id, nil
}

func bindJSON(c *gin.Context, target any) error {
	if err := c.ShouldBindJSON(target); err != nil {
		return apperror.Wrap(err, apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效")
	}
	return nil
}

func (handler *Handler) listCategories(c *gin.Context) {
	filter, err := pageFilter(c)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	page, err := handler.service.ListCategories(c.Request.Context(), filter)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	items := make([]categoryResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, toCategoryResponse(item))
	}
	managementresponse.OK(c, managementresponse.Page{List: items, Total: page.Total, Page: page.Page, PageSize: page.PageSize}, "获取成功")
}

func (handler *Handler) createCategory(c *gin.Context) {
	var request categoryRequest
	if err := bindJSON(c, &request); err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	item, err := handler.service.CreateCategory(c.Request.Context(), CategoryInput{Code: request.Code, Name: request.Name, Kind: request.Kind, WorkDirection: request.WorkDirection, Sort: request.Sort, Enabled: *request.Enabled})
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, toCategoryResponse(item), "创建成功")
}

func (handler *Handler) updateCategory(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	var request categoryRequest
	if err := bindJSON(c, &request); err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	item, err := handler.service.UpdateCategory(c.Request.Context(), id, CategoryInput{Code: request.Code, Name: request.Name, Kind: request.Kind, WorkDirection: request.WorkDirection, Sort: request.Sort, Enabled: *request.Enabled})
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, toCategoryResponse(item), "更新成功")
}

func (handler *Handler) deleteCategory(c *gin.Context) {
	id, err := pathID(c)
	if err == nil {
		err = handler.service.DeleteCategory(c.Request.Context(), id)
	}
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.Message(c, "删除成功")
}

func (handler *Handler) listAuthors(c *gin.Context) {
	filter, err := pageFilter(c)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	page, err := handler.service.ListAuthors(c.Request.Context(), filter)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	items := make([]authorResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, toAuthorResponse(item))
	}
	managementresponse.OK(c, managementresponse.Page{List: items, Total: page.Total, Page: page.Page, PageSize: page.PageSize}, "获取成功")
}

func (handler *Handler) createAuthor(c *gin.Context) {
	var request authorRequest
	if err := bindJSON(c, &request); err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	item, err := handler.service.CreateAuthor(c.Request.Context(), AuthorInput{PenName: request.PenName, Status: request.Status, WorkDirection: request.WorkDirection})
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, toAuthorResponse(item), "创建成功")
}

func (handler *Handler) updateAuthor(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	var request authorRequest
	if err := bindJSON(c, &request); err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	item, err := handler.service.UpdateAuthor(c.Request.Context(), id, AuthorInput{PenName: request.PenName, Status: request.Status, WorkDirection: request.WorkDirection})
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, toAuthorResponse(item), "更新成功")
}

func (handler *Handler) deleteAuthor(c *gin.Context) {
	id, err := pathID(c)
	if err == nil {
		err = handler.service.DeleteAuthor(c.Request.Context(), id)
	}
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.Message(c, "删除成功")
}

func toCategoryResponse(item Category) categoryResponse {
	return categoryResponse{ID: strconv.FormatInt(item.ID, 10), Code: item.Code, Name: item.Name, Kind: item.Kind, WorkDirection: item.WorkDirection, Sort: item.Sort, Enabled: item.Enabled, Source: item.Source, CreatedAt: item.CreatedAt.Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.Format(time.RFC3339Nano)}
}

func toAuthorResponse(item Author) authorResponse {
	return authorResponse{ID: strconv.FormatInt(item.ID, 10), PenName: item.PenName, Status: item.Status, WorkDirection: item.WorkDirection, Source: item.Source, LegacyAuthorID: formatOptionalID(item.LegacyAuthorID), LegacyBookAuthorID: formatOptionalID(item.LegacyBookAuthorID), CreatedAt: item.CreatedAt.Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.Format(time.RFC3339Nano)}
}

func formatOptionalID(id *int64) *string {
	if id == nil {
		return nil
	}
	value := strconv.FormatInt(*id, 10)
	return &value
}
