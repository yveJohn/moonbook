package books

import (
	"database/sql"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

const maxCoverBytes = 10 << 20

type Handler struct {
	service *Service
	objects *objectstore.Service
}

type request struct {
	WorkDirection    *string  `json:"workDirection"`
	CategoryCode     string   `json:"categoryCode" binding:"required"`
	LegacyCoverURL   string   `json:"legacyCoverUrl"`
	BookName         string   `json:"bookName" binding:"required"`
	AuthorID         string   `json:"authorId" binding:"required"`
	Description      string   `json:"description"`
	Score            string   `json:"score" binding:"required"`
	BookStatus       string   `json:"bookStatus" binding:"required"`
	PublishStatus    string   `json:"publishStatus" binding:"required"`
	SourceType       string   `json:"sourceType" binding:"required"`
	Featured         *bool    `json:"featured" binding:"required"`
	FeaturedSort     int      `json:"featuredSort"`
	FeaturedNote     string   `json:"featuredNote"`
	ChargeMode       string   `json:"chargeMode" binding:"required"`
	FixedPriceCoin   *string  `json:"fixedPriceCoin"`
	SubCategoryCodes []string `json:"subCategoryCodes"`
	Tags             []string `json:"tags"`
}

type categoryResponse struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}
type response struct {
	ID                   string             `json:"id"`
	WorkDirection        *string            `json:"workDirection"`
	PrimaryCategory      categoryResponse   `json:"primaryCategory"`
	LegacyCoverURL       string             `json:"legacyCoverUrl"`
	BookName             string             `json:"bookName"`
	AuthorID             string             `json:"authorId"`
	AuthorName           string             `json:"authorName"`
	Description          string             `json:"description"`
	Score                string             `json:"score"`
	BookStatus           string             `json:"bookStatus"`
	PublishStatus        string             `json:"publishStatus"`
	SourceType           string             `json:"sourceType"`
	Featured             bool               `json:"featured"`
	FeaturedSort         int                `json:"featuredSort"`
	FeaturedNote         string             `json:"featuredNote"`
	VisitCount           int64              `json:"visitCount"`
	LikeCount            int                `json:"likeCount"`
	WordCount            int                `json:"wordCount"`
	CommentCount         int                `json:"commentCount"`
	YesterdayBuy         int                `json:"yesterdayBuy"`
	LastChapterID        *string            `json:"lastChapterId"`
	LastChapterName      *string            `json:"lastChapterName"`
	LastChapterUpdatedAt *string            `json:"lastChapterUpdatedAt"`
	ChargeMode           string             `json:"chargeMode"`
	FixedPriceCoin       *string            `json:"fixedPriceCoin"`
	SubCategories        []categoryResponse `json:"subCategories"`
	Tags                 []string           `json:"tags"`
	CreatedAt            string             `json:"createdAt"`
	UpdatedAt            string             `json:"updatedAt"`
}

type coverResponse struct {
	ObjectID string `json:"objectId"`
	Version  int    `json:"version"`
	SHA256   string `json:"sha256"`
	ByteSize int64  `json:"byteSize"`
}

func RegisterRoutes(private *gin.RouterGroup, db *sql.DB, objects *objectstore.Service) {
	h := &Handler{service: NewService(db), objects: objects}
	read := private.Group("novel")
	write := private.Group("novel").Use(middleware.OperationRecord())
	read.GET("books", h.list)
	read.GET("books/:id", h.get)
	read.GET("books/:id/cover", h.getCover)
	write.POST("books", h.create)
	write.PUT("books/:id", h.update)
	write.DELETE("books/:id", h.delete)
	write.POST("books/:id/cover", h.uploadCover)
}

func parsePositiveLong(raw, field string) (int64, error) {
	if raw == "" || strings.TrimSpace(raw) != raw || strings.HasPrefix(raw, "+") {
		return 0, invalid(field + "必须是十进制正整数字符串")
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, invalid(field + "必须是十进制正整数字符串")
	}
	return value, nil
}

func parseOptionalLong(raw *string, field string) (*int64, error) {
	if raw == nil {
		return nil, nil
	}
	value, err := parsePositiveLong(*raw, field)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func bind(c *gin.Context) (Input, error) {
	var r request
	if err := c.ShouldBindJSON(&r); err != nil {
		return Input{}, apperror.Wrap(err, apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效")
	}
	authorID, err := parsePositiveLong(r.AuthorID, "作者 ID")
	if err != nil {
		return Input{}, err
	}
	price, err := parseOptionalLong(r.FixedPriceCoin, "整书售价")
	if err != nil {
		return Input{}, err
	}
	return Input{WorkDirection: r.WorkDirection, CategoryCode: r.CategoryCode, LegacyCoverURL: r.LegacyCoverURL, BookName: r.BookName, AuthorID: authorID, Description: r.Description, Score: r.Score, BookStatus: r.BookStatus, PublishStatus: r.PublishStatus, SourceType: r.SourceType, Featured: *r.Featured, FeaturedSort: r.FeaturedSort, FeaturedNote: r.FeaturedNote, ChargeMode: r.ChargeMode, FixedPriceCoin: price, SubCategoryCodes: r.SubCategoryCodes, Tags: r.Tags}, nil
}

func listFilter(c *gin.Context) (Filter, error) {
	page, err := optionalPositiveInt(c.Query("page"), 1)
	if err != nil {
		return Filter{}, invalid("page 必须是正整数")
	}
	pageSize, err := optionalPositiveInt(c.Query("pageSize"), 10)
	if err != nil {
		return Filter{}, invalid("pageSize 必须是正整数")
	}
	return Filter{
		Page: page, PageSize: pageSize, Keyword: c.Query("keyword"),
		CategoryCode: c.Query("categoryCode"), BookStatus: c.Query("bookStatus"),
		PublishStatus: c.Query("publishStatus"), SourceType: c.Query("sourceType"),
		ChargeMode: c.Query("chargeMode"),
	}, nil
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

func (h *Handler) list(c *gin.Context) {
	filter, err := listFilter(c)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	result, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	items := make([]response, 0, len(result.Items))
	for _, book := range result.Items {
		items = append(items, toResponse(book))
	}
	managementresponse.OK(c, managementresponse.Page{List: items, Total: result.Total, Page: result.Page, PageSize: result.PageSize}, "获取成功")
}
func (h *Handler) get(c *gin.Context) {
	id, err := parsePositiveLong(c.Param("id"), "书籍 ID")
	if err == nil {
		var book Book
		book, err = h.service.Get(c.Request.Context(), id)
		if err == nil {
			managementresponse.OK(c, toResponse(book), "获取成功")
			return
		}
	}
	apperror.WriteManagement(c, err)
}
func (h *Handler) create(c *gin.Context) {
	input, err := bind(c)
	if err == nil {
		var book Book
		book, err = h.service.Create(c.Request.Context(), input)
		if err == nil {
			managementresponse.OK(c, toResponse(book), "创建成功")
			return
		}
	}
	apperror.WriteManagement(c, err)
}
func (h *Handler) update(c *gin.Context) {
	id, err := parsePositiveLong(c.Param("id"), "书籍 ID")
	var input Input
	if err == nil {
		input, err = bind(c)
	}
	if err == nil {
		var book Book
		book, err = h.service.Update(c.Request.Context(), id, input)
		if err == nil {
			managementresponse.OK(c, toResponse(book), "更新成功")
			return
		}
	}
	apperror.WriteManagement(c, err)
}
func (h *Handler) delete(c *gin.Context) {
	id, err := parsePositiveLong(c.Param("id"), "书籍 ID")
	if err == nil {
		err = h.service.Delete(c.Request.Context(), id)
	}
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.Message(c, "删除成功")
}

func (h *Handler) uploadCover(c *gin.Context) {
	id, err := parsePositiveLong(c.Param("id"), "书籍 ID")
	if err == nil {
		_, err = h.service.Get(c.Request.Context(), id)
	}
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCoverBytes+(1<<20))
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		apperror.WriteManagement(c, invalid("请选择不超过 10 MiB 的封面文件"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxCoverBytes+1))
	if err != nil || len(data) == 0 || len(data) > maxCoverBytes {
		apperror.WriteManagement(c, invalid("封面文件必须为 1 字节至 10 MiB"))
		return
	}
	contentType, extension, ok := objectstore.DetectCover(data)
	if !ok {
		apperror.WriteManagement(c, invalid("封面仅支持 JPEG、PNG、WebP 或 GIF"))
		return
	}
	_ = header
	object, err := h.objects.UploadVerified(c.Request.Context(), objectstore.Target{Kind: objectstore.KindBookCover, BookID: id, OwnerID: id, Extension: extension}, data, contentType)
	if err == nil {
		err = h.objects.Activate(c.Request.Context(), object.ID)
	}
	if err != nil {
		apperror.WriteManagement(c, apperror.Wrap(err, apperror.CodeUnavailable, http.StatusServiceUnavailable, "封面存储暂不可用"))
		return
	}
	managementresponse.OK(c, coverResponse{ObjectID: strconv.FormatInt(object.ID, 10), Version: object.Version, SHA256: object.SHA256, ByteSize: object.ByteSize}, "封面上传成功")
}

func (h *Handler) getCover(c *gin.Context) {
	id, err := parsePositiveLong(c.Param("id"), "书籍 ID")
	if err == nil {
		_, err = h.service.Get(c.Request.Context(), id)
	}
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	data, object, err := h.objects.ReadActive(c.Request.Context(), objectstore.Target{Kind: objectstore.KindBookCover, BookID: id, OwnerID: id, Extension: "png"})
	if err != nil {
		if errors.Is(err, objectstore.ErrActiveObjectNotFound) {
			apperror.WriteManagement(c, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "书籍封面不存在"))
		} else {
			apperror.WriteManagement(c, apperror.Wrap(err, apperror.CodeUnavailable, http.StatusServiceUnavailable, "封面存储暂不可用"))
		}
		return
	}
	c.Header("Content-Type", object.ContentType)
	c.Header("ETag", `"`+object.SHA256+`"`)
	c.Header("Cache-Control", "private, max-age=300")
	c.Data(http.StatusOK, object.ContentType, data)
}

func coverType(data []byte) (string, string, bool) {
	return objectstore.DetectCover(data)
}

func toResponse(book Book) response {
	subs := make([]categoryResponse, 0, len(book.SubCategories))
	for _, c := range book.SubCategories {
		subs = append(subs, categoryResponse{ID: strconv.FormatInt(c.ID, 10), Code: c.Code, Name: c.Name})
	}
	return response{ID: strconv.FormatInt(book.ID, 10), WorkDirection: book.WorkDirection, PrimaryCategory: categoryResponse{ID: strconv.FormatInt(book.PrimaryCategory.ID, 10), Code: book.PrimaryCategory.Code, Name: book.PrimaryCategory.Name}, LegacyCoverURL: book.LegacyCoverURL, BookName: book.BookName, AuthorID: strconv.FormatInt(book.AuthorID, 10), AuthorName: book.AuthorName, Description: book.Description, Score: book.Score, BookStatus: book.BookStatus, PublishStatus: book.PublishStatus, SourceType: book.SourceType, Featured: book.Featured, FeaturedSort: book.FeaturedSort, FeaturedNote: book.FeaturedNote, VisitCount: book.VisitCount, LikeCount: book.LikeCount, WordCount: book.WordCount, CommentCount: book.CommentCount, YesterdayBuy: book.YesterdayBuy, LastChapterID: formatID(book.LastChapterID), LastChapterName: book.LastChapterName, LastChapterUpdatedAt: formatTime(book.LastChapterUpdatedAt), ChargeMode: book.ChargeMode, FixedPriceCoin: formatID(book.FixedPriceCoin), SubCategories: subs, Tags: book.Tags, CreatedAt: book.CreatedAt.Format(time.RFC3339Nano), UpdatedAt: book.UpdatedAt.Format(time.RFC3339Nano)}
}
func formatID(value *int64) *string {
	if value == nil {
		return nil
	}
	result := strconv.FormatInt(*value, 10)
	return &result
}
func formatTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	result := value.Format(time.RFC3339Nano)
	return &result
}
