package commercecompat

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	readerwire "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/wire"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/gin-gonic/gin"
)

type purchaseResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func RegisterPurchaseRoutes(group *gin.RouterGroup, service commercecontract.PurchaseReader, auth *readerauth.Service) {
	handler := purchaseHandler{service: service}
	routes := group.Group("/reader/me").Use(readerauth.RequireReader(auth))
	routes.POST("/orders/membership", handler.membership)
	routes.POST("/orders/chapter", handler.chapter)
	routes.POST("/orders/book", handler.book)
}

type purchaseHandler struct {
	service commercecontract.PurchaseReader
}

func purchaseFail(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, commercecontract.ErrQuoteChanged):
		ctx.JSON(http.StatusOK, purchaseResponse{Code: 46106, Msg: "作品报价已变化，请确认新价格"})
		return
	case errors.Is(err, commercecontract.ErrInsufficientFunds):
		ctx.JSON(http.StatusOK, purchaseResponse{Code: 500, Msg: "余额不足"})
		return
	case errors.Is(err, commercecontract.ErrInvalidRequest):
		err = apperror.New(apperror.CodeInvalidArgument, 200, "购买参数无效")
	case errors.Is(err, commercecontract.ErrProductUnavailable):
		err = apperror.New(apperror.CodeNotFound, 200, "购买商品不可用")
	}
	public := apperror.Expose(err)
	code := 500
	if public.Code == apperror.CodeUnauthenticated {
		code = 401
	}
	if public.Code == apperror.CodeInvalidArgument || public.Code == apperror.CodeNotFound {
		code = 200
	}
	ctx.JSON(http.StatusOK, purchaseResponse{Code: code, Msg: public.Message})
}

func positiveID(value string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, commercecontract.ErrInvalidRequest
	}
	return parsed, nil
}

func longValue(raw json.RawMessage) (int64, error) {
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return positiveID(value)
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil {
		return positiveID(number.String())
	}
	return 0, commercecontract.ErrInvalidRequest
}

func purchaseOrderJSON(order commercecontract.PurchaseOrder) map[string]any {
	return map[string]any{
		"id": optionalLong(order.ID), "orderNo": nullableString(order.OrderNo), "readerId": strconv.FormatInt(order.ReaderID, 10),
		"orderType": order.OrderType, "productId": optionalLongPointer(order.ProductID), "productType": order.ProductType,
		"targetId": optionalLongPointer(order.TargetID), "bookIdSnapshot": optionalLongPointer(order.BookIDSnapshot),
		"productNameSnapshot": nullableString(order.ProductName), "priceCoinSnapshot": optionalLong(order.PriceCoin),
		"chapterWordCountSnapshot": order.ChapterWordCount, "pricingWordUnitSnapshot": order.PricingWordUnit,
		"pricingCoinUnitSnapshot": optionalLongPointer(order.PricingCoinUnit), "rechargeCoinAmount": strconv.FormatInt(order.RechargeCoinAmount, 10),
		"bonusCoinAmount": strconv.FormatInt(order.BonusCoinAmount, 10), "status": order.Status,
		"idempotencyKey": order.IdempotencyKey, "remark": order.Remark, "operatorId": optionalLongPointer(order.OperatorID),
		"paidTime": readerwire.DateTimePointer(order.PaidAt), "createTime": readerwire.DateTime(order.CreatedAt),
		"updateTime": readerwire.DateTime(order.UpdatedAt),
	}
}

func optionalLong(value int64) any {
	if value == 0 {
		return nil
	}
	return strconv.FormatInt(value, 10)
}

func optionalLongPointer(value *int64) any {
	if value == nil || *value == 0 {
		return nil
	}
	return strconv.FormatInt(*value, 10)
}

func (handler purchaseHandler) membership(ctx *gin.Context) {
	var input struct {
		ProductID string `json:"productId"`
		RequestID string `json:"requestId"`
	}
	if ctx.ShouldBindJSON(&input) != nil {
		purchaseFail(ctx, commercecontract.ErrInvalidRequest)
		return
	}
	productID, err := positiveID(input.ProductID)
	if err != nil {
		purchaseFail(ctx, err)
		return
	}
	id, err := readerID(ctx)
	if err != nil {
		purchaseFail(ctx, err)
		return
	}
	order, err := handler.service.BuyMembership(ctx, id, productID, input.RequestID)
	if err != nil {
		purchaseFail(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, purchaseResponse{Code: 200, Msg: "购买成功", Data: purchaseOrderJSON(order)})
}

func (handler purchaseHandler) chapter(ctx *gin.Context) {
	var input struct {
		ChapterID     string          `json:"chapterId"`
		ExpectedPrice json.RawMessage `json:"expectedPrice"`
		RequestID     string          `json:"requestId"`
	}
	if ctx.ShouldBindJSON(&input) != nil {
		purchaseFail(ctx, commercecontract.ErrInvalidRequest)
		return
	}
	chapterID, err := positiveID(input.ChapterID)
	if err != nil {
		purchaseFail(ctx, err)
		return
	}
	price, err := longValue(input.ExpectedPrice)
	if err != nil {
		purchaseFail(ctx, err)
		return
	}
	id, err := readerID(ctx)
	if err != nil {
		purchaseFail(ctx, err)
		return
	}
	result, err := handler.service.BuyChapter(ctx, id, chapterID, price, input.RequestID)
	if err != nil {
		purchaseFail(ctx, err)
		return
	}
	data := map[string]any{"purchaseStatus": result.Status, "quote": map[string]any{
		"chapterId": strconv.FormatInt(result.Quote.ChapterID, 10), "bookId": strconv.FormatInt(result.Quote.BookID, 10),
		"wordCount": result.Quote.WordCount, "wordUnit": result.Quote.WordUnit,
		"coinUnit": strconv.FormatInt(result.Quote.CoinUnit, 10), "priceCoin": strconv.FormatInt(result.Quote.PriceCoin, 10),
	}, "order": nil}
	if result.Order != nil {
		data["order"] = purchaseOrderJSON(*result.Order)
	}
	ctx.JSON(http.StatusOK, purchaseResponse{Code: 200, Msg: "购买成功", Data: data})
}

func (handler purchaseHandler) book(ctx *gin.Context) {
	var input struct {
		BookID        string          `json:"bookId"`
		ExpectedPrice json.RawMessage `json:"expectedPrice"`
	}
	if ctx.ShouldBindJSON(&input) != nil {
		purchaseFail(ctx, commercecontract.ErrInvalidRequest)
		return
	}
	bookID, err := positiveID(input.BookID)
	if err != nil {
		purchaseFail(ctx, err)
		return
	}
	price, err := longValue(input.ExpectedPrice)
	if err != nil {
		purchaseFail(ctx, err)
		return
	}
	id, err := readerID(ctx)
	if err != nil {
		purchaseFail(ctx, err)
		return
	}
	order, err := handler.service.BuyBook(ctx, id, bookID, price)
	if err != nil {
		purchaseFail(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, purchaseResponse{Code: 200, Msg: "购买成功", Data: purchaseOrderJSON(order)})
}
