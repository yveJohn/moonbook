package wallet

import (
	"net/http"
	"strconv"

	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/gin-gonic/gin"
)

type response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}
type pageResponse struct {
	Code  int    `json:"code"`
	Msg   string `json:"msg"`
	Rows  any    `json:"rows"`
	Total int64  `json:"total"`
}

func RegisterRoutes(group *gin.RouterGroup, service *Service, auth *readerauth.Service) {
	h := &Handler{service: service}
	r := group.Group("/reader/me").Use(readerauth.RequireReader(auth))
	r.GET("/wallet", h.get)
	r.GET("/wallet/ledgers", h.ledgers)
}

type Handler struct{ service *Service }

func readerID(c *gin.Context) (int64, error) {
	id, ok := readerauth.ReaderIdentity(c)
	if !ok {
		return 0, readerauth.ErrInvalidSession
	}
	return id.ReaderID, nil
}
func walletFail(c *gin.Context, err error) {
	p := apperror.Expose(err)
	code := 500
	if p.Code == apperror.CodeUnauthenticated {
		code = 401
	}
	if p.Code == apperror.CodeInvalidArgument {
		code = 200
	}
	c.JSON(http.StatusOK, response{Code: code, Msg: p.Message})
}
func (h *Handler) get(c *gin.Context) {
	id, err := readerID(c)
	if err != nil {
		walletFail(c, err)
		return
	}
	v, err := h.service.Get(c, id)
	if err != nil {
		walletFail(c, err)
		return
	}
	ok := map[string]any{"readerId": strconv.FormatInt(v.ReaderID, 10), "rechargeCoinBalance": strconv.FormatInt(v.RechargeCoinBalance, 10), "bonusCoinBalance": strconv.FormatInt(v.BonusCoinBalance, 10), "totalRechargeCoinIncome": strconv.FormatInt(v.TotalRechargeCoinIncome, 10), "totalBonusCoinIncome": strconv.FormatInt(v.TotalBonusCoinIncome, 10), "totalRechargeCoinExpense": strconv.FormatInt(v.TotalRechargeCoinExpense, 10), "totalBonusCoinExpense": strconv.FormatInt(v.TotalBonusCoinExpense, 10), "expiringBonusCoin": "0"}
	c.JSON(http.StatusOK, response{Code: 200, Msg: "查询成功", Data: ok})
}
func (h *Handler) ledgers(c *gin.Context) {
	id, err := readerID(c)
	if err != nil {
		walletFail(c, err)
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("pageNum", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	coin := c.DefaultQuery("coinType", "recharge")
	rows, total, err := h.service.List(c, id, coin, page, size)
	if err != nil {
		walletFail(c, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, v := range rows {
		out = append(out, map[string]any{"id": strconv.FormatInt(v.ID, 10), "readerId": strconv.FormatInt(v.ReaderID, 10), "ledgerNo": v.LedgerNo, "bizType": v.BizType, "bizId": v.BizID, "orderNo": v.OrderNo, "direction": v.Direction, "coinType": v.CoinType, "amount": strconv.FormatInt(v.Amount, 10), "balanceBefore": strconv.FormatInt(v.BalanceBefore, 10), "balanceAfter": strconv.FormatInt(v.BalanceAfter, 10), "remark": v.Remark, "createTime": v.CreatedAt})
	}
	c.JSON(http.StatusOK, pageResponse{Code: 200, Msg: "查询成功", Rows: out, Total: total})
}
