package txtimport

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
	store   FileStore
}

func RegisterRoutes(private *gin.RouterGroup, service *Service, store FileStore) {
	h := &Handler{service: service, store: store}
	read := private.Group("novel/txtImports")
	write := private.Group("novel/txtImports").Use(middleware.OperationRecord())
	read.GET("", h.list)
	read.GET("/:id/preview", h.preview)
	read.GET("/:id", h.get)
	write.POST("", h.upload)
	write.POST("/:id/file", h.replaceFile)
	write.POST("/:id/retry", h.retry)
	write.POST("/:id/cancel", h.cancel)
}

func render(v Task) map[string]any {
	return map[string]any{"id": v.ID, "targetBookId": v.TargetBookID, "originalFilename": v.OriginalFilename, "objectByteSize": v.ObjectByteSize, "status": v.Status, "qualityStatus": v.QualityStatus, "qualitySummary": v.QualitySummary, "totalChapterCount": v.TotalChapterCount, "importedChapterCount": v.ImportedChapterCount, "emptyChapterCount": v.EmptyChapterCount, "duplicateChapterCount": v.DuplicateChapterCount, "failReason": v.FailReason, "operatorName": v.OperatorName, "attemptCount": v.AttemptCount, "maxAttempts": v.MaxAttempts, "startTime": v.StartTime, "endTime": v.EndTime, "createdAt": v.CreatedAt, "updatedAt": v.UpdatedAt}
}

func parseID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串")
	}
	return id, nil
}

func (h *Handler) list(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	rows, total, err := h.service.List(c, c.Query("keyword"), c.Query("status"), page, size)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	items := make([]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, render(row))
	}
	managementresponse.OK(c, managementresponse.Page{List: items, Total: total, Page: page, PageSize: size}, "获取成功")
}

func (h *Handler) get(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err == nil {
		var item Task
		item, err = h.service.Get(c, id)
		if err == nil {
			managementresponse.OK(c, render(item), "获取成功")
			return
		}
	}
	apperror.WriteManagement(c, err)
}

func (h *Handler) preview(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err == nil {
		item, previewErr := h.service.Preview(c, h.store, id)
		if previewErr == nil {
			managementresponse.OK(c, item, "预览成功")
			return
		}
		err = previewErr
	}
	apperror.WriteManagement(c, err)
}

func (h *Handler) upload(c *gin.Context) {
	if h.store == nil {
		apperror.WriteManagement(c, errors.New("TXT file store is not configured"))
		return
	}
	bookID := c.PostForm("targetBookId")
	operator := c.PostForm("operatorName")
	filename, data, err := readUpload(c)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	item, err := h.service.Upload(c, h.store, bookID, filename, operator, data)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, render(item), "上传成功，已排队")
}

func readUpload(c *gin.Context) (string, []byte, error) {
	file, err := c.FormFile("file")
	if err != nil {
		return "", nil, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "TXT 文件不能为空")
	}
	if file.Size <= 0 || file.Size > MaxFileBytes {
		return "", nil, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "TXT 文件大小必须在 1 字节到 16 MiB 之间")
	}
	opened, err := file.Open()
	if err != nil {
		return "", nil, err
	}
	defer opened.Close()
	data, err := io.ReadAll(io.LimitReader(opened, MaxFileBytes+1))
	if err != nil || int64(len(data)) > MaxFileBytes {
		return "", nil, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "TXT 文件读取失败或超过大小限制")
	}
	return file.Filename, data, nil
}

func (h *Handler) replaceFile(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	var filename string
	var data []byte
	if err == nil {
		filename, data, err = readUpload(c)
	}
	if err == nil {
		item, replaceErr := h.service.ReplaceFile(c, h.store, id, filename, data)
		if replaceErr == nil {
			managementresponse.OK(c, render(item), "文件已替换并重新排队")
			return
		}
		err = replaceErr
	}
	apperror.WriteManagement(c, err)
}

func (h *Handler) retry(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err == nil {
		var item Task
		item, err = h.service.Retry(c, id)
		if err == nil {
			managementresponse.OK(c, render(item), "已重新排队")
			return
		}
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
