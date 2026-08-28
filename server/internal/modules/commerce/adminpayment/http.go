package adminpayment

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func RegisterRoutes(private *gin.RouterGroup, service *Service) {
	h := &Handler{service: service}
	read := private.Group("reader/payment")
	write := private.Group("reader/payment").Use(middleware.OperationRecord())
	read.GET("channels", h.list)
	read.GET("channels/:id", h.get)
	write.POST("channels", h.create)
	write.PUT("channels/:id", h.update)
	write.DELETE("channels/:id", h.archive)
	write.POST("channels/:id/check", h.check)
}

type channelRequest struct {
	DisplayName           string  `json:"displayName"`
	Provider              string  `json:"provider"`
	Enabled               bool    `json:"enabled"`
	Currency              string  `json:"currency"`
	Token                 string  `json:"token"`
	Network               string  `json:"network"`
	MerchantPID           *string `json:"merchantPid"`
	Secret                *string `json:"secret"`
	EPUSDTBaseURL         string  `json:"epusdtBaseUrl"`
	ReaderBaseURL         string  `json:"readerBaseUrl"`
	ConnectTimeoutMS      int     `json:"connectTimeoutMs"`
	RequestTimeoutMS      int     `json:"requestTimeoutMs"`
	UnknownReleaseMinutes int     `json:"unknownReleaseMinutes"`
}

func (request channelRequest) input() ChannelInput {
	return ChannelInput{
		DisplayName: strings.TrimSpace(request.DisplayName), Provider: strings.ToLower(strings.TrimSpace(request.Provider)), Enabled: request.Enabled,
		Currency: strings.ToLower(strings.TrimSpace(request.Currency)), Token: strings.ToLower(strings.TrimSpace(request.Token)), Network: strings.ToLower(strings.TrimSpace(request.Network)),
		MerchantPID: request.MerchantPID, Secret: request.Secret, EPUSDTBaseURL: request.EPUSDTBaseURL, ReaderBaseURL: request.ReaderBaseURL,
		ConnectTimeoutMS: request.ConnectTimeoutMS, RequestTimeoutMS: request.RequestTimeoutMS, UnknownReleaseMinutes: request.UnknownReleaseMinutes,
	}
}

func (h *Handler) list(c *gin.Context) {
	rows, err := h.service.List(c, c.Query("includeArchived") == "true")
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	out := make([]any, 0, len(rows))
	for _, item := range rows {
		out = append(out, render(item))
	}
	managementresponse.OK(c, out, "获取成功")
}

func (h *Handler) get(c *gin.Context) {
	id, ok := channelID(c)
	if !ok {
		return
	}
	item, err := h.service.Get(c, id)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, render(item), "获取成功")
}

func (h *Handler) create(c *gin.Context) {
	var request channelRequest
	if c.ShouldBindJSON(&request) != nil {
		apperror.WriteManagement(c, invalid("支付渠道参数无效"))
		return
	}
	item, err := h.service.Create(c, request.input())
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, render(item), "新增成功")
}

func (h *Handler) update(c *gin.Context) {
	id, ok := channelID(c)
	if !ok {
		return
	}
	var request channelRequest
	if c.ShouldBindJSON(&request) != nil {
		apperror.WriteManagement(c, invalid("支付渠道参数无效"))
		return
	}
	item, err := h.service.Update(c, id, request.input())
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, render(item), "更新成功")
}

func (h *Handler) archive(c *gin.Context) {
	id, ok := channelID(c)
	if !ok {
		return
	}
	if err := h.service.Archive(c, id); err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, nil, "归档成功")
}

func (h *Handler) check(c *gin.Context) {
	id, ok := channelID(c)
	if !ok {
		return
	}
	value, err := h.service.Check(c, id)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, map[string]any{"channelId": strconv.FormatInt(value.ChannelID, 10), "provider": value.Provider, "status": value.Status, "message": value.Message, "checkedAt": value.CheckedAt}, "检查完成")
}

func channelID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串"))
		return 0, false
	}
	return id, true
}

func render(item Channel) map[string]any {
	endpoints := item.Endpoints()
	return map[string]any{
		"id": strconv.FormatInt(item.ID, 10), "displayName": item.DisplayName, "provider": item.Provider, "enabled": item.Enabled,
		"currency": item.Currency, "token": item.Token, "network": item.Network, "configured": item.Configured(),
		"pidConfigured": item.PIDConfigured, "secretConfigured": item.SecretConfigured,
		"epusdtBaseUrl": item.EPUSDTBaseURL, "readerBaseUrl": item.ReaderBaseURL,
		"createUrl": endpoints.CreateURL, "notifyUrl": endpoints.NotifyURL, "redirectUrl": endpoints.RedirectURL, "healthUrl": endpoints.HealthURL, "syncUrl": endpoints.SyncURL,
		"connectTimeoutMs": item.ConnectTimeoutMS, "requestTimeoutMs": item.RequestTimeoutMS, "unknownReleaseMinutes": item.UnknownReleaseMinutes,
		"archivedAt": item.ArchivedAt, "createdAt": item.CreatedAt, "updatedAt": item.UpdatedAt,
	}
}
