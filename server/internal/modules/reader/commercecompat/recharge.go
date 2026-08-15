package commercecompat

import (
	"errors"
	"net/http"
	"strconv"

	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	readerwire "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/wire"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/gin-gonic/gin"
)

type rechargeResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func RegisterRechargeRoutes(group *gin.RouterGroup, service commercecontract.RechargeReader, auth *readerauth.Service) {
	handler := rechargeHandler{service: service}
	public := group.Group("/reader/recharge")
	public.GET("/products", handler.catalog)
	public.POST("/quote", handler.quote)
	private := group.Group("/reader/me").Use(readerauth.RequireReader(auth))
	private.POST("/recharge/orders", handler.create)
	private.GET("/recharge/orders/:orderId", handler.get)
}

type rechargeHandler struct {
	service commercecontract.RechargeReader
}

func rechargeFail(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, commercecontract.ErrInvalidRequest):
		ctx.JSON(http.StatusOK, rechargeResponse{Code: 500, Msg: "充值参数无效"})
		return
	case errors.Is(err, commercecontract.ErrProductUnavailable):
		ctx.JSON(http.StatusOK, rechargeResponse{Code: 500, Msg: "充值档位不存在或已下架"})
		return
	case errors.Is(err, commercecontract.ErrNotFound):
		ctx.JSON(http.StatusOK, rechargeResponse{Code: 500, Msg: "充值订单不存在"})
		return
	}
	public := apperror.Expose(err)
	code := 500
	if public.Code == apperror.CodeUnauthenticated {
		code = 401
	}
	if public.Code == apperror.CodeInvalidArgument || public.Code == apperror.CodeNotFound {
		code = 200
	}
	ctx.JSON(http.StatusOK, rechargeResponse{Code: code, Msg: public.Message})
}

func (handler rechargeHandler) catalog(ctx *gin.Context) {
	value, err := handler.service.RechargeCatalog(ctx)
	if err != nil {
		rechargeFail(ctx, err)
		return
	}
	products := make([]map[string]any, 0, len(value.Products))
	for _, product := range value.Products {
		products = append(products, map[string]any{
			"id": strconv.FormatInt(product.ID, 10), "productName": product.Name,
			"diamondAmount": strconv.FormatInt(product.DiamondAmount, 10), "priceUsdt": product.PriceUSDT,
			"saleStatus": product.SaleStatus, "sortOrder": product.SortOrder,
		})
	}
	ctx.JSON(http.StatusOK, rechargeResponse{Code: 200, Msg: "查询成功", Data: map[string]any{
		"products": products, "customEnabled": value.CustomEnabled, "diamondsPerUsdt": value.DiamondsPerUSDT,
		"minDiamondAmount": value.MinDiamondAmount, "maxDiamondAmount": value.MaxDiamondAmount,
	}})
}

func (handler rechargeHandler) quote(ctx *gin.Context) {
	var input struct {
		DiamondAmount string `json:"diamondAmount"`
	}
	if ctx.ShouldBindJSON(&input) != nil {
		rechargeFail(ctx, commercecontract.ErrInvalidRequest)
		return
	}
	amount, err := positiveID(input.DiamondAmount)
	if err != nil {
		rechargeFail(ctx, commercecontract.ErrInvalidRequest)
		return
	}
	quote, err := handler.service.QuoteRecharge(ctx, amount)
	if err != nil {
		rechargeFail(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, rechargeResponse{Code: 200, Msg: "查询成功", Data: map[string]any{"diamondAmount": strconv.FormatInt(quote.DiamondAmount, 10), "priceUsdt": quote.PriceUSDT}})
}

func (handler rechargeHandler) create(ctx *gin.Context) {
	var input struct {
		ProductID           string `json:"productId"`
		CustomDiamondAmount string `json:"customDiamondAmount"`
		RequestID           string `json:"requestId"`
	}
	if ctx.ShouldBindJSON(&input) != nil {
		rechargeFail(ctx, commercecontract.ErrInvalidRequest)
		return
	}
	id, err := readerID(ctx)
	if err != nil {
		rechargeFail(ctx, err)
		return
	}
	request := commercecontract.RechargeCreateRequest{ReaderID: id, RequestID: input.RequestID}
	if input.ProductID != "" {
		value, parseErr := positiveID(input.ProductID)
		if parseErr != nil {
			rechargeFail(ctx, commercecontract.ErrInvalidRequest)
			return
		}
		request.ProductID = &value
	}
	if input.CustomDiamondAmount != "" {
		value, parseErr := positiveID(input.CustomDiamondAmount)
		if parseErr != nil {
			rechargeFail(ctx, commercecontract.ErrInvalidRequest)
			return
		}
		request.CustomDiamondAmount = &value
	}
	order, err := handler.service.CreateRechargeOrder(ctx, request)
	if err != nil {
		rechargeFail(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, rechargeResponse{Code: 200, Msg: "订单创建成功", Data: rechargeOrderJSON(order)})
}

func (handler rechargeHandler) get(ctx *gin.Context) {
	id, err := readerID(ctx)
	if err != nil {
		rechargeFail(ctx, err)
		return
	}
	orderID, err := positiveID(ctx.Param("orderId"))
	if err != nil {
		rechargeFail(ctx, commercecontract.ErrUnavailable)
		return
	}
	order, err := handler.service.RechargeOrder(ctx, id, orderID)
	if err != nil {
		rechargeFail(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, rechargeResponse{Code: 200, Msg: "查询成功", Data: rechargeOrderJSON(order)})
}

func rechargeOrderJSON(order commercecontract.RechargeOrder) map[string]any {
	return map[string]any{
		"id": strconv.FormatInt(order.ID, 10), "orderNo": order.OrderNo, "readerId": strconv.FormatInt(order.ReaderID, 10),
		"sourceType": order.SourceType, "productId": optionalLongPointer(order.ProductID),
		"diamondAmount": strconv.FormatInt(order.DiamondAmount, 10), "priceUsdt": order.PriceUSDT,
		"provider": order.Provider, "currency": order.Currency, "token": order.Token, "network": order.Network,
		"gatewayTradeId": order.GatewayTradeID, "actualAmount": order.ActualAmount, "receiveAddress": order.ReceiveAddress,
		"paymentUrl": order.PaymentURL, "blockTransactionId": order.BlockTransactionID, "status": order.Status,
		"gatewayStatus": order.GatewayStatus, "walletLedgerId": optionalLongPointer(order.WalletLedgerID),
		"expireTime": readerwire.DateTimePointer(order.ExpiresAt), "paidTime": readerwire.DateTimePointer(order.PaidAt),
		"failureCode": order.FailureCode, "failureMessage": order.FailureMessage,
		"createTime": readerwire.DateTime(order.CreatedAt), "updateTime": readerwire.DateTime(order.UpdatedAt),
	}
}
