package adminwallet

import (
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type Handler struct{ service *Service }

func RegisterRoutes(private *gin.RouterGroup, service *Service) {
	h := &Handler{service: service}
	r := private.Group("reader")
	w := private.Group("reader").Use(middleware.OperationRecord())
	r.GET("wallets", h.wallets)
	r.GET("wallets/:readerId/ledgers", h.ledgers)
	w.POST("wallets/:readerId/adjust", h.adjust)
}
func (h *Handler) adjust(c *gin.Context) {
	readerID := c.Param("readerId")
	var req struct{ Amount, CoinType, Direction, Reason, RequestID string }
	if c.ShouldBindJSON(&req) != nil {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效"))
		return
	}
	v, err := h.service.Adjust(c, AdjustmentInput{ReaderID: readerID, Amount: req.Amount, CoinType: req.CoinType, Direction: req.Direction, Reason: req.Reason, RequestID: req.RequestID})
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, ledgerOut(v.Ledger), "调账成功")
}
func walletOut(v Wallet) map[string]any {
	return map[string]any{"readerId": v.ReaderID, "readerUsername": v.ReaderUsername, "rechargeBalance": v.RechargeBalance, "bonusBalance": v.BonusBalance, "totalRechargeIncome": v.TotalRechargeIncome, "totalBonusIncome": v.TotalBonusIncome, "totalRechargeExpense": v.TotalRechargeExpense, "totalBonusExpense": v.TotalBonusExpense}
}
func ledgerOut(v Ledger) map[string]any {
	return map[string]any{"id": v.ID, "readerId": v.ReaderID, "ledgerNo": v.LedgerNo, "bizType": v.BizType, "direction": v.Direction, "coinType": v.CoinType, "amount": v.Amount, "balanceBefore": v.BalanceBefore, "balanceAfter": v.BalanceAfter, "bizId": v.BizID, "orderNo": v.OrderNo, "remark": v.Remark, "createdAt": v.CreatedAt}
}
func (h *Handler) wallets(c *gin.Context) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	n, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	rows, total, err := h.service.ListWallets(c, c.Query("keyword"), p, n)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	items := make([]any, 0, len(rows))
	for _, v := range rows {
		items = append(items, walletOut(v))
	}
	managementresponse.OK(c, managementresponse.Page{List: items, Total: total, Page: p, PageSize: n}, "获取成功")
}
func (h *Handler) ledgers(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("readerId"), 10, 64)
	if err != nil || id <= 0 {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "readerId必须是正整数字符串"))
		return
	}
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	n, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	rows, total, err := h.service.ListLedgers(c, id, c.Query("coinType"), p, n)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	items := make([]any, 0, len(rows))
	for _, v := range rows {
		items = append(items, ledgerOut(v))
	}
	managementresponse.OK(c, managementresponse.Page{List: items, Total: total, Page: p, PageSize: n}, "获取成功")
}
