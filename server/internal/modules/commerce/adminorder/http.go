package adminorder

import (
	"net/http"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/flipped-aurora/gin-vue-admin/server/utils"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

func RegisterRoutes(private *gin.RouterGroup, service *Service) {
	h := &Handler{service: service}
	r := private.Group("reader")
	r.GET("orders", h.list)
	r.GET("orders/:id", h.get)
	write := private.Group("reader").Use(middleware.OperationRecord())
	write.POST("orders/mockRecharge", h.createMockRecharge)
	write.POST("orders/:id/confirmMockRecharge", h.confirmMockRecharge)
}

func (h *Handler) createMockRecharge(c *gin.Context) {
	var in MockRechargeInput
	if c.ShouldBindJSON(&in) != nil {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效"))
		return
	}
	o, err := h.service.CreateMockRecharge(c, in, int64(utils.GetUserID(c)))
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, out(o), "模拟充值订单已创建")
}

func (h *Handler) confirmMockRecharge(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串"))
		return
	}
	o, err := h.service.ConfirmMockRecharge(c, id, int64(utils.GetUserID(c)))
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, out(o), "模拟充值已确认")
}

func out(o Order) map[string]any {
	return map[string]any{"id": o.ID, "readerId": o.ReaderID, "readerUsername": o.ReaderUsername, "orderNo": o.OrderNo, "orderType": o.OrderType, "productId": o.ProductID, "productType": o.ProductType, "targetId": o.TargetID, "bookIdSnapshot": o.BookIDSnapshot, "productName": o.ProductName, "priceCoin": o.PriceCoin, "chapterWordCount": o.ChapterWordCount, "pricingWordUnit": o.PricingWordUnit, "pricingCoinUnit": o.PricingCoinUnit, "rechargeCoinAmount": o.RechargeCoinAmount, "bonusCoinAmount": o.BonusCoinAmount, "status": o.Status, "remark": o.Remark, "paidAt": o.PaidAt, "createdAt": o.CreatedAt, "updatedAt": o.UpdatedAt}
}
func (h *Handler) list(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	items, total, err := h.service.List(c, c.Query("keyword"), c.Query("orderType"), c.Query("status"), page, size)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	rows := make([]any, 0, len(items))
	for _, o := range items {
		rows = append(rows, out(o))
	}
	managementresponse.OK(c, managementresponse.Page{List: rows, Total: total, Page: page, PageSize: size}, "获取成功")
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
	managementresponse.OK(c, out(o), "获取成功")
}
