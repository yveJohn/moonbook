package recharge

import (
	"net/http"
	"strconv"

	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	readerwire "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/wire"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/gin-gonic/gin"
)

type response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}
type Handler struct{ service *Service }

func RegisterRoutes(group *gin.RouterGroup, service *Service, auth *readerauth.Service) {
	h := &Handler{service: service}
	public := group.Group("/reader/recharge")
	public.GET("/products", h.catalog)
	public.POST("/quote", h.quote)
	private := group.Group("/reader/me").Use(readerauth.RequireReader(auth))
	private.POST("/recharge/orders", h.create)
	private.GET("/recharge/orders/:orderId", h.get)
}
func rid(c *gin.Context) (int64, error) {
	x, ok := readerauth.ReaderIdentity(c)
	if !ok {
		return 0, readerauth.ErrInvalidSession
	}
	return x.ReaderID, nil
}
func fail(c *gin.Context, e error) {
	p := apperror.Expose(e)
	code := 500
	if p.Code == apperror.CodeUnauthenticated {
		code = 401
	}
	if p.Code == apperror.CodeInvalidArgument || p.Code == apperror.CodeNotFound {
		code = 200
	}
	c.JSON(http.StatusOK, response{Code: code, Msg: p.Message})
}
func (h *Handler) catalog(c *gin.Context) {
	v, e := h.service.Catalog(c)
	if e != nil {
		fail(c, e)
		return
	}
	products := make([]map[string]any, 0, len(v.Products))
	for _, p := range v.Products {
		products = append(products, map[string]any{"id": strconv.FormatInt(p.ID, 10), "productName": p.ProductName, "diamondAmount": strconv.FormatInt(p.DiamondAmount, 10), "priceUsdt": p.PriceUSDT, "saleStatus": p.SaleStatus, "sortOrder": p.SortOrder})
	}
	c.JSON(http.StatusOK, response{Code: 200, Msg: "查询成功", Data: map[string]any{"products": products, "customEnabled": v.CustomEnabled, "diamondsPerUsdt": v.DiamondsPerUSDT, "minDiamondAmount": v.MinDiamondAmount, "maxDiamondAmount": v.MaxDiamondAmount}})
}
func (h *Handler) quote(c *gin.Context) {
	var req struct {
		DiamondAmount string `json:"diamondAmount"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, ErrInvalidRecharge)
		return
	}
	n, e := strconv.ParseInt(req.DiamondAmount, 10, 64)
	if e != nil {
		fail(c, ErrInvalidRecharge)
		return
	}
	v, e := h.service.Quote(c, n)
	if e != nil {
		fail(c, e)
		return
	}
	c.JSON(http.StatusOK, response{Code: 200, Msg: "查询成功", Data: map[string]any{"diamondAmount": v.DiamondAmount, "priceUsdt": v.PriceUSDT}})
}
func (h *Handler) create(c *gin.Context) {
	var req struct {
		ProductID           string `json:"productId"`
		CustomDiamondAmount string `json:"customDiamondAmount"`
		RequestID           string `json:"requestId"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, ErrInvalidRecharge)
		return
	}
	readerID, e := rid(c)
	if e != nil {
		fail(c, e)
		return
	}
	in := CreateRequest{ReaderID: readerID, CustomDiamondAmount: req.CustomDiamondAmount, RequestID: req.RequestID}
	if req.ProductID != "" {
		id, err := strconv.ParseInt(req.ProductID, 10, 64)
		if err != nil {
			fail(c, ErrInvalidRecharge)
			return
		}
		in.ProductID = &id
	}
	v, e := h.service.CreateOrder(c, in)
	if e != nil {
		fail(c, e)
		return
	}
	c.JSON(http.StatusOK, response{Code: 200, Msg: "订单创建成功", Data: orderJSON(v)})
}
func (h *Handler) get(c *gin.Context) {
	readerID, e := rid(c)
	if e != nil {
		fail(c, e)
		return
	}
	v, e := h.service.GetOrder(c, readerID, c.Param("orderId"))
	if e != nil {
		fail(c, e)
		return
	}
	c.JSON(http.StatusOK, response{Code: 200, Msg: "查询成功", Data: orderJSON(v)})
}
func orderJSON(v Order) map[string]any {
	return map[string]any{"id": v.ID, "orderNo": v.OrderNo, "readerId": v.ReaderID, "sourceType": v.SourceType, "productId": nullIfEmpty(v.ProductID), "diamondAmount": v.DiamondAmount, "priceUsdt": v.PriceUSDT, "provider": v.Provider, "currency": v.Currency, "token": v.Token, "network": v.Network, "gatewayTradeId": v.GatewayTradeID, "actualAmount": v.ActualAmount, "receiveAddress": v.ReceiveAddress, "paymentUrl": v.PaymentURL, "blockTransactionId": v.BlockTransactionID, "status": v.Status, "gatewayStatus": v.GatewayStatus, "walletLedgerId": v.WalletLedgerID, "expireTime": readerwire.DateTimePointer(v.ExpireTime), "paidTime": readerwire.DateTimePointer(v.PaidTime), "failureCode": v.FailureCode, "failureMessage": v.FailureMessage, "createTime": readerwire.DateTime(v.CreateTime), "updateTime": readerwire.DateTime(v.UpdateTime)}
}
func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}
