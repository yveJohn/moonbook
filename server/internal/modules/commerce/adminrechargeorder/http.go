package adminrechargeorder

import (
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type Handler struct{ service *Service }

func RegisterRoutes(private *gin.RouterGroup, service *Service) {
	h := &Handler{service: service}
	r := private.Group("reader/payment")
	r.GET("orders", h.list)
	r.GET("orders/:id", h.get)
	r.GET("callbackLogs", h.callbackList)
	r.GET("callbackLogs/:id", h.callbackGet)
}

func renderCallback(v CallbackLog) map[string]any {
	return map[string]any{"id": v.ID, "orderId": v.OrderID, "provider": v.Provider, "merchantOrderNo": v.MerchantOrderNo, "gatewayTradeId": v.GatewayTradeID, "payloadHash": v.PayloadHash, "signatureValid": v.SignatureValid, "processingResult": v.ProcessingResult, "failureReason": v.FailureReason, "responseStatus": v.ResponseStatus, "responseBody": v.ResponseBody, "requestTime": v.RequestTime, "createdAt": v.CreatedAt}
}

func (h *Handler) callbackList(c *gin.Context) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	n, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	rows, total, err := h.service.ListCallbacks(c, c.Query("keyword"), c.Query("processingResult"), p, n)
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
	return map[string]any{"id": o.ID, "readerId": o.ReaderID, "readerUsername": o.ReaderUsername, "diamondAmount": o.DiamondAmount, "productId": o.ProductID, "orderNo": o.OrderNo, "sourceType": o.SourceType, "priceUsdt": o.PriceUSDT, "provider": o.Provider, "currency": o.Currency, "token": o.Token, "network": o.Network, "gatewayTradeId": o.GatewayTradeID, "actualAmount": o.ActualAmount, "receiveAddress": o.ReceiveAddress, "paymentUrl": o.PaymentURL, "blockTransactionId": o.BlockTransactionID, "status": o.Status, "gatewayStatus": o.GatewayStatus, "createdAt": o.CreatedAt, "paidAt": o.PaidAt}
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
