package purchase

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
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
	r := group.Group("/reader/me").Use(readerauth.RequireReader(auth))
	r.POST("/orders/membership", h.membership)
	r.POST("/orders/chapter", h.chapter)
	r.POST("/orders/book", h.book)
}
func purchaseReaderID(c *gin.Context) (int64, error) {
	x, ok := readerauth.ReaderIdentity(c)
	if !ok {
		return 0, readerauth.ErrInvalidSession
	}
	return x.ReaderID, nil
}
func purchaseFail(c *gin.Context, e error) {
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
func longString(raw json.RawMessage) (string, error) {
	var s string
	if json.Unmarshal(raw, &s) == nil && s != "" {
		return s, nil
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil && n.String() != "" {
		return n.String(), nil
	}
	return "", ErrInvalidRequest
}
func orderJSON(o Order) map[string]any {
	return map[string]any{"id": emptyNull(o.ID), "orderNo": emptyNull(o.OrderNo), "readerId": o.ReaderID, "orderType": o.OrderType, "productId": emptyNull(o.ProductID), "productType": o.ProductType, "targetId": emptyNull(o.TargetID), "bookIdSnapshot": emptyNull(o.BookIDSnapshot), "productNameSnapshot": emptyNull(o.ProductName), "priceCoinSnapshot": emptyNull(o.PriceCoin), "chapterWordCountSnapshot": numberNull(o.ChapterWordCount), "pricingWordUnitSnapshot": numberNull(o.PricingWordUnit), "pricingCoinUnitSnapshot": emptyNull(o.PricingCoinUnit), "rechargeCoinAmount": o.RechargeCoinAmount, "bonusCoinAmount": o.BonusCoinAmount, "status": o.Status, "idempotencyKey": o.IdempotencyKey, "remark": o.Remark, "operatorId": emptyNull(o.OperatorID), "paidTime": datePointer(o.PaidTime), "createTime": dateValue(o.CreateTime), "updateTime": dateValue(o.UpdateTime)}
}
func emptyNull(s string) any {
	if s == "" || s == "0" {
		return nil
	}
	return s
}
func numberNull(s string) any {
	if s == "" || s == "0" {
		return nil
	}
	n, _ := strconv.Atoi(s)
	return n
}
func datePointer(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format("2006-01-02 15:04:05")
}
func dateValue(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC().Format("2006-01-02 15:04:05")
}
func (h *Handler) membership(c *gin.Context) {
	var in struct {
		ProductID string `json:"productId"`
		RequestID string `json:"requestId"`
	}
	if c.ShouldBindJSON(&in) != nil {
		purchaseFail(c, ErrInvalidRequest)
		return
	}
	id, e := purchaseReaderID(c)
	if e != nil {
		purchaseFail(c, e)
		return
	}
	o, e := h.service.BuyMembership(c, id, in.ProductID, in.RequestID)
	if e != nil {
		purchaseFail(c, e)
		return
	}
	c.JSON(http.StatusOK, response{Code: 200, Msg: "购买成功", Data: orderJSON(o)})
}
func (h *Handler) chapter(c *gin.Context) {
	var in struct {
		ChapterID     string          `json:"chapterId"`
		ExpectedPrice json.RawMessage `json:"expectedPrice"`
		RequestID     string          `json:"requestId"`
	}
	if c.ShouldBindJSON(&in) != nil {
		purchaseFail(c, ErrInvalidRequest)
		return
	}
	price, e := longString(in.ExpectedPrice)
	if e != nil {
		purchaseFail(c, e)
		return
	}
	id, e := purchaseReaderID(c)
	if e != nil {
		purchaseFail(c, e)
		return
	}
	v, e := h.service.BuyChapter(c, id, in.ChapterID, price, in.RequestID)
	if e != nil {
		purchaseFail(c, e)
		return
	}
	data := map[string]any{"purchaseStatus": v.PurchaseStatus, "quote": map[string]any{"chapterId": v.Quote.ChapterID, "bookId": v.Quote.BookID, "wordCount": v.Quote.WordCount, "wordUnit": v.Quote.WordUnit, "coinUnit": v.Quote.CoinUnit, "priceCoin": v.Quote.PriceCoin}, "order": nil}
	if v.Order != nil {
		data["order"] = orderJSON(*v.Order)
	}
	c.JSON(http.StatusOK, response{Code: 200, Msg: "购买成功", Data: data})
}
func (h *Handler) book(c *gin.Context) {
	var in struct {
		BookID        string          `json:"bookId"`
		ExpectedPrice json.RawMessage `json:"expectedPrice"`
	}
	if c.ShouldBindJSON(&in) != nil {
		purchaseFail(c, ErrInvalidRequest)
		return
	}
	price, e := longString(in.ExpectedPrice)
	if e != nil {
		purchaseFail(c, e)
		return
	}
	id, e := purchaseReaderID(c)
	if e != nil {
		purchaseFail(c, e)
		return
	}
	o, e := h.service.BuyBook(c, id, in.BookID, price)
	if e != nil {
		purchaseFail(c, e)
		return
	}
	c.JSON(http.StatusOK, response{Code: 200, Msg: "购买成功", Data: orderJSON(o)})
}
