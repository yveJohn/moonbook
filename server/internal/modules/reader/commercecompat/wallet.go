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

type walletResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

type walletPageResponse struct {
	Code  int    `json:"code"`
	Msg   string `json:"msg"`
	Rows  any    `json:"rows"`
	Total int64  `json:"total"`
}

func RegisterWalletRoutes(group *gin.RouterGroup, service commercecontract.WalletReader, auth *readerauth.Service) {
	handler := walletHandler{service: service}
	routes := group.Group("/reader/me").Use(readerauth.RequireReader(auth))
	routes.GET("/wallet", handler.get)
	routes.GET("/wallet/ledgers", handler.ledgers)
}

type walletHandler struct{ service commercecontract.WalletReader }

func walletFail(ctx *gin.Context, err error) {
	public := apperror.Expose(err)
	code := 500
	if public.Code == apperror.CodeUnauthenticated {
		code = 401
	}
	if errors.Is(err, commercecontract.ErrInvalidRequest) || public.Code == apperror.CodeInvalidArgument {
		code = 200
	}
	ctx.JSON(http.StatusOK, walletResponse{Code: code, Msg: public.Message})
}

func (handler walletHandler) get(ctx *gin.Context) {
	id, err := readerID(ctx)
	if err != nil {
		walletFail(ctx, err)
		return
	}
	value, err := handler.service.Wallet(ctx, id)
	if err != nil {
		walletFail(ctx, err)
		return
	}
	data := map[string]any{
		"readerId": strconv.FormatInt(value.ReaderID, 10), "rechargeCoinBalance": strconv.FormatInt(value.RechargeCoinBalance, 10),
		"bonusCoinBalance": strconv.FormatInt(value.BonusCoinBalance, 10), "totalRechargeCoinIncome": strconv.FormatInt(value.TotalRechargeCoinIncome, 10),
		"totalBonusCoinIncome": strconv.FormatInt(value.TotalBonusCoinIncome, 10), "totalRechargeCoinExpense": strconv.FormatInt(value.TotalRechargeCoinExpense, 10),
		"totalBonusCoinExpense": strconv.FormatInt(value.TotalBonusCoinExpense, 10), "expiringBonusCoin": "0",
	}
	ctx.JSON(http.StatusOK, walletResponse{Code: 200, Msg: "查询成功", Data: data})
}

func (handler walletHandler) ledgers(ctx *gin.Context) {
	id, err := readerID(ctx)
	if err != nil {
		walletFail(ctx, err)
		return
	}
	page, _ := strconv.Atoi(ctx.DefaultQuery("pageNum", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	coinType := ctx.DefaultQuery("coinType", "recharge")
	rows, total, err := handler.service.WalletLedgers(ctx, id, coinType, page, size)
	if err != nil {
		walletFail(ctx, err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, value := range rows {
		out = append(out, map[string]any{
			"id": strconv.FormatInt(value.ID, 10), "readerId": strconv.FormatInt(value.ReaderID, 10),
			"ledgerNo": value.LedgerNo, "bizType": value.BizType, "bizId": value.BizID,
			"orderNo": value.OrderNo, "direction": value.Direction, "coinType": value.CoinType,
			"amount": strconv.FormatInt(value.Amount, 10), "balanceBefore": strconv.FormatInt(value.BalanceBefore, 10),
			"balanceAfter": strconv.FormatInt(value.BalanceAfter, 10), "remark": value.Remark,
			"createTime": readerwire.DateTime(value.CreatedAt),
		})
	}
	ctx.JSON(http.StatusOK, walletPageResponse{Code: 200, Msg: "查询成功", Rows: out, Total: total})
}
