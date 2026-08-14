package readerseo

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

type configRequest struct {
	SEOEnabled               *bool  `json:"seoEnabled" binding:"required"`
	IndexingEnabled          *bool  `json:"indexingEnabled" binding:"required"`
	SitemapEnabled           *bool  `json:"sitemapEnabled" binding:"required"`
	SiteName                 string `json:"siteName" binding:"required"`
	SiteURL                  string `json:"siteUrl" binding:"required"`
	DefaultDescription       string `json:"defaultDescription" binding:"required"`
	HomeTitle                string `json:"homeTitle" binding:"required"`
	HomeDescription          string `json:"homeDescription" binding:"required"`
	BooksTitleTemplate       string `json:"booksTitleTemplate" binding:"required"`
	BooksDescriptionTemplate string `json:"booksDescriptionTemplate" binding:"required"`
	BookTitleTemplate        string `json:"bookTitleTemplate" binding:"required"`
	BookDescriptionTemplate  string `json:"bookDescriptionTemplate" binding:"required"`
}

type configResponse struct {
	ID                       string `json:"id"`
	SEOEnabled               bool   `json:"seoEnabled"`
	IndexingEnabled          bool   `json:"indexingEnabled"`
	SitemapEnabled           bool   `json:"sitemapEnabled"`
	SiteName                 string `json:"siteName"`
	SiteURL                  string `json:"siteUrl"`
	DefaultDescription       string `json:"defaultDescription"`
	HomeTitle                string `json:"homeTitle"`
	HomeDescription          string `json:"homeDescription"`
	BooksTitleTemplate       string `json:"booksTitleTemplate"`
	BooksDescriptionTemplate string `json:"booksDescriptionTemplate"`
	BookTitleTemplate        string `json:"bookTitleTemplate"`
	BookDescriptionTemplate  string `json:"bookDescriptionTemplate"`
	CreatedAt                string `json:"createdAt"`
	UpdatedAt                string `json:"updatedAt"`
}

func RegisterRoutes(private *gin.RouterGroup, db *sql.DB) {
	handler := &Handler{service: NewService(db)}
	read := private.Group("novel")
	write := private.Group("novel").Use(middleware.OperationRecord())
	read.GET("readerSeo/config", handler.get)
	write.PUT("readerSeo/config", handler.update)
}

func (handler *Handler) get(c *gin.Context) {
	config, err := handler.service.Get(c.Request.Context())
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, toResponse(config), "获取成功")
}

func (handler *Handler) update(c *gin.Context) {
	var request configRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		apperror.WriteManagement(c, apperror.Wrap(err, apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效"))
		return
	}
	config, err := handler.service.Update(c.Request.Context(), Input{
		SEOEnabled: *request.SEOEnabled, IndexingEnabled: *request.IndexingEnabled,
		SitemapEnabled: *request.SitemapEnabled, SiteName: request.SiteName, SiteURL: request.SiteURL,
		DefaultDescription: request.DefaultDescription, HomeTitle: request.HomeTitle,
		HomeDescription: request.HomeDescription, BooksTitleTemplate: request.BooksTitleTemplate,
		BooksDescriptionTemplate: request.BooksDescriptionTemplate, BookTitleTemplate: request.BookTitleTemplate,
		BookDescriptionTemplate: request.BookDescriptionTemplate,
	})
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, toResponse(config), "保存成功")
}

func toResponse(config Config) configResponse {
	return configResponse{
		ID: strconv.FormatInt(config.ID, 10), SEOEnabled: config.SEOEnabled,
		IndexingEnabled: config.IndexingEnabled, SitemapEnabled: config.SitemapEnabled,
		SiteName: config.SiteName, SiteURL: config.SiteURL, DefaultDescription: config.DefaultDescription,
		HomeTitle: config.HomeTitle, HomeDescription: config.HomeDescription,
		BooksTitleTemplate: config.BooksTitleTemplate, BooksDescriptionTemplate: config.BooksDescriptionTemplate,
		BookTitleTemplate: config.BookTitleTemplate, BookDescriptionTemplate: config.BookDescriptionTemplate,
		CreatedAt: config.CreatedAt.Format(time.RFC3339Nano), UpdatedAt: config.UpdatedAt.Format(time.RFC3339Nano),
	}
}
