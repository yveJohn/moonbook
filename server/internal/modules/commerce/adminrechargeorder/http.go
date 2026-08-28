package adminrechargeorder

import (
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

type Handler struct{ service *Service }

func RegisterRoutes(private *gin.RouterGroup, service *Service) {
	h := &Handler{service: service}
	r := private.Group("reader/payment")
	w := private.Group("reader/payment").Use(middleware.OperationRecord())
	r.GET("orders", h.list)
	r.GET("orders/:id", h.get)
	w.POST("orders/:id/manualPay", h.manualPay)
	w.POST("orders/:id/sync", h.sync)
	r.GET("callbackLogs", h.callbackList)
	r.GET("callbackLogs/:id", h.callbackGet)
}

func (h *Handler) manualPay(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串"))
		return
	}
	var req ManualPayInput
	if c.ShouldBindJSON(&req) != nil {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效"))
		return
	}
	o, err := h.service.ManualPay(c, id, req)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, render(o), "人工补单成功")
}

func (h *Handler) sync(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串"))
		return
	}
	o, err := h.service.Sync(c, id)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	message := "同步完成"
	if o.Status == "callback_exception" && o.GatewayStatus != nil && *o.GatewayStatus == 2 {
		message = "网关已支付但缺少有效回调，请在GM Pay重发回调"
	}
	managementresponse.OK(c, render(o), message)
}

func renderCallback(v CallbackLog) map[string]any {
	return map[string]any{
		"id": v.ID, "orderId": v.OrderID, "provider": v.Provider, "merchantOrderNo": v.MerchantOrderNo,
		"gatewayTradeId": v.GatewayTradeID, "payloadHash": v.PayloadHash, "signatureValid": v.SignatureValid,
		"signatureStatus": v.SignatureStatus, "processingResult": v.ProcessingResult, "failureCode": v.FailureCode,
		"failureReason": v.FailureReason, "responseStatus": v.ResponseStatus, "responseBody": v.ResponseBody,
		"requestId": v.RequestID, "traceId": v.TraceID, "payloadBytes": v.PayloadBytes,
		"payloadTruncated": v.PayloadTruncated, "requestTime": v.RequestTime, "createdAt": v.CreatedAt,
		"completedAt": v.CompletedAt, "interrupted": v.Interrupted, "sourceType": v.SourceType, "snapshot": v.Snapshot,
	}
}

func (h *Handler) callbackList(c *gin.Context) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	n, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if p < 1 {
		p = 1
	}
	if n < 1 {
		n = 20
	}
	if n > 100 {
		n = 100
	}
	filter, err := callbackFilterFromQuery(c)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	filter.Page, filter.PageSize = p, n
	rows, total, err := h.service.ListCallbacks(c, filter)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	items := make([]any, 0, len(rows))
	for _, v := range rows {
		items = append(items, renderCallback(v))
	}
	managementresponse.OK(c, managementresponse.Page{List: items, Total: total, Page: p, PageSize: n}, "获取成功")
}

func callbackFilterFromQuery(c *gin.Context) (CallbackFilter, error) {
	filter := CallbackFilter{
		Keyword: c.Query("keyword"), ProcessingResult: c.Query("processingResult"), SignatureStatus: c.Query("signatureStatus"), FailureCode: c.Query("failureCode"),
	}
	if raw := c.Query("responseStatus"); raw != "" {
		status, err := strconv.Atoi(raw)
		if err != nil {
			return CallbackFilter{}, invalidCallbackFilter("响应状态无效")
		}
		filter.ResponseStatus = &status
	}
	for raw, destination := range map[string]**time.Time{"startTime": &filter.StartTime, "endTime": &filter.EndTime} {
		value := c.Query(raw)
		if value == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return CallbackFilter{}, invalidCallbackFilter("时间参数必须包含明确时区")
		}
		*destination = &parsed
	}
	if err := validateCallbackFilter(filter); err != nil {
		return CallbackFilter{}, err
	}
	return filter, nil
}

func (h *Handler) callbackGet(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串"))
		return
	}
	v, err := h.service.GetCallback(c, id)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, renderCallback(v), "获取成功")
}
func render(o Order) map[string]any {
	return map[string]any{"id": o.ID, "readerId": o.ReaderID, "readerUsername": o.ReaderUsername, "diamondAmount": o.DiamondAmount, "productId": o.ProductID, "orderNo": o.OrderNo, "sourceType": o.SourceType, "priceUsdt": o.PriceUSDT, "provider": o.Provider, "currency": o.Currency, "token": o.Token, "network": o.Network, "gatewayTradeId": o.GatewayTradeID, "actualAmount": o.ActualAmount, "receiveAddress": o.ReceiveAddress, "paymentUrl": o.PaymentURL, "blockTransactionId": o.BlockTransactionID, "status": o.Status, "failureCode": o.FailureCode, "failureMessage": o.FailureMessage, "gatewayStatus": o.GatewayStatus, "createdAt": o.CreatedAt, "paidAt": o.PaidAt}
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
	for _, o := range rows {
		items = append(items, render(o))
	}
	managementresponse.OK(c, managementresponse.Page{List: items, Total: total, Page: p, PageSize: n}, "获取成功")
}
func (h *Handler) get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串"))
		return
	}
	o, err := h.service.Get(c, id)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, render(o), "获取成功")
}
