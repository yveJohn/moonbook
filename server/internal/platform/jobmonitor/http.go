package jobmonitor

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func RegisterRoutes(private *gin.RouterGroup, service *Service) {
	h := &Handler{service: service}
	g := private.Group("platform/jobs")
	g.GET("", h.list)
	g.GET("/:id", h.get)
}

func renderJob(job Job) map[string]any {
	return map[string]any{
		"id": job.ID, "module": job.Module, "jobType": job.JobType, "status": job.Status,
		"attemptCount": job.AttemptCount, "maxAttempts": job.MaxAttempts, "availableAt": job.AvailableAt,
		"leaseOwner": job.LeaseOwner, "leaseExpiresAt": job.LeaseExpiresAt, "lastErrorCode": job.LastErrorCode,
		"lastErrorMessage": sanitizeMessage(job.LastErrorMessage), "createdAt": job.CreatedAt,
		"updatedAt": job.UpdatedAt, "finishedAt": job.FinishedAt,
	}
}

func renderAttempt(attempt Attempt) map[string]any {
	return map[string]any{
		"id": attempt.ID, "jobId": attempt.JobID, "attemptNumber": attempt.AttemptNumber, "workerId": attempt.WorkerID,
		"startedAt": attempt.StartedAt, "finishedAt": attempt.FinishedAt, "outcome": attempt.Outcome,
		"errorCode": attempt.ErrorCode, "errorMessage": sanitizeMessage(attempt.ErrorMessage),
	}
}

func (h *Handler) list(c *gin.Context) {
	page, size, err := parsePagination(c.Query("page"), c.Query("pageSize"))
	if err != nil {
		writeError(c, err)
		return
	}
	filters := Filters{Module: c.Query("module"), JobType: c.Query("jobType"), Status: c.Query("status"), LeaseOwner: c.Query("leaseOwner"), From: c.Query("from"), To: c.Query("to")}
	rows, total, err := h.service.List(c, filters, page, size)
	if err != nil {
		writeError(c, err)
		return
	}
	items := make([]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, renderJob(row))
	}
	managementresponse.OK(c, managementresponse.Page{List: items, Total: total, Page: page, PageSize: size}, "获取成功")
}

func (h *Handler) get(c *gin.Context) {
	detail, err := h.service.Get(c, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	attempts := make([]any, 0, len(detail.Attempts))
	for _, attempt := range detail.Attempts {
		attempts = append(attempts, renderAttempt(attempt))
	}
	data := renderJob(detail.Job)
	data["attempts"] = attempts
	managementresponse.OK(c, data, "获取成功")
}

func parsePagination(rawPage, rawSize string) (int, int, error) {
	page, size := 1, 20
	var err error
	if rawPage != "" {
		page, err = strconv.Atoi(rawPage)
		if err != nil || page < 1 {
			return 0, 0, fmt.Errorf("%w: page must be a positive integer", ErrInvalidArgument)
		}
	}
	if rawSize != "" {
		size, err = strconv.Atoi(rawSize)
		if err != nil || size < 1 || size > 100 {
			return 0, 0, fmt.Errorf("%w: pageSize must be between 1 and 100", ErrInvalidArgument)
		}
	}
	return page, size, nil
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidArgument):
		apperror.WriteManagement(c, apperror.Wrap(err, apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数不正确"))
	case errors.Is(err, ErrNotFound):
		apperror.WriteManagement(c, apperror.Wrap(err, apperror.CodeNotFound, http.StatusNotFound, "平台任务不存在"))
	default:
		apperror.WriteManagement(c, err)
	}
}
