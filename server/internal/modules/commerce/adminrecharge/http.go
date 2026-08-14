package adminrecharge

import (
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type Handler struct{ service *Service }
type productRequest struct {
	ProductName   string `json:"productName" binding:"required"`
	DiamondAmount string `json:"diamondAmount" binding:"required"`
	PriceUSDT     string `json:"priceUsdt" binding:"required"`
	SaleStatus    string `json:"saleStatus" binding:"required"`
	SortOrder     int    `json:"sortOrder"`
}

func RegisterRoutes(private *gin.RouterGroup, service *Service) {
	h := &Handler{service: service}
	read := private.Group("reader/payment")
	write := private.Group("reader/payment").Use(middleware.OperationRecord())
	read.GET("rechargeProducts", h.list)
	read.GET("rechargeSettings", h.setting)
	write.POST("rechargeProducts", h.create)
	write.PUT("rechargeSettings", h.updateSetting)
	write.PUT("rechargeProducts/:id", h.update)
	write.DELETE("rechargeProducts/:id", h.delete)
}
func settingOut(v Setting) map[string]any {
	return map[string]any{"id": strconv.FormatInt(v.ID, 10), "customEnabled": v.CustomEnabled, "diamondsPerUsdt": v.DiamondsPerUSDT, "minDiamondAmount": v.MinDiamondAmount, "maxDiamondAmount": v.MaxDiamondAmount, "amountScale": v.AmountScale, "roundingMode": v.RoundingMode}
}
func (h *Handler) setting(c *gin.Context) {
	v, err := h.service.GetSetting(c)
	if err != nil {
		fail(c, err)
		return
	}
	managementresponse.OK(c, settingOut(v), "获取成功")
}
func (h *Handler) updateSetting(c *gin.Context) {
	var req struct {
		CustomEnabled    bool   `json:"customEnabled"`
		DiamondsPerUSDT  string `json:"diamondsPerUsdt"`
		MinDiamondAmount string `json:"minDiamondAmount"`
		MaxDiamondAmount string `json:"maxDiamondAmount"`
	}
	if c.ShouldBindJSON(&req) != nil {
		fail(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效"))
		return
	}
	v, err := h.service.UpdateSetting(c, SettingInput{CustomEnabled: req.CustomEnabled, DiamondsPerUSDT: req.DiamondsPerUSDT, MinDiamondAmount: req.MinDiamondAmount, MaxDiamondAmount: req.MaxDiamondAmount})
	if err != nil {
		fail(c, err)
		return
	}
	managementresponse.OK(c, settingOut(v), "保存成功")
}
func input(r productRequest) Input {
	return Input{ProductName: r.ProductName, DiamondAmount: r.DiamondAmount, PriceUSDT: r.PriceUSDT, SaleStatus: r.SaleStatus, SortOrder: r.SortOrder}
}
func out(v Product) map[string]any {
	return map[string]any{"id": strconv.FormatInt(v.ID, 10), "productName": v.ProductName, "diamondAmount": strconv.FormatInt(v.DiamondAmount, 10), "priceUsdt": v.PriceUSDT, "saleStatus": v.SaleStatus, "sortOrder": v.SortOrder}
}
func fail(c *gin.Context, e error) { apperror.WriteManagement(c, e) }
func (h *Handler) list(c *gin.Context) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	n, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	rows, total, e := h.service.List(c, c.Query("keyword"), p, n)
	if e != nil {
		fail(c, e)
		return
	}
	items := make([]any, 0, len(rows))
	for _, v := range rows {
		items = append(items, out(v))
	}
	managementresponse.OK(c, managementresponse.Page{List: items, Total: total, Page: p, PageSize: n}, "获取成功")
}
func (h *Handler) create(c *gin.Context) {
	var r productRequest
	if c.ShouldBindJSON(&r) != nil {
		fail(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效"))
		return
	}
	v, e := h.service.Create(c, input(r))
	if e != nil {
		fail(c, e)
		return
	}
	managementresponse.OK(c, out(v), "创建成功")
}
func id(c *gin.Context) (int64, error) {
	v, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil || v <= 0 {
		return 0, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串")
	}
	return v, nil
}
func (h *Handler) update(c *gin.Context) {
	id, e := id(c)
	if e != nil {
		fail(c, e)
		return
	}
	var r productRequest
	if c.ShouldBindJSON(&r) != nil {
		fail(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效"))
		return
	}
	v, e := h.service.Update(c, id, input(r))
	if e != nil {
		fail(c, e)
		return
	}
	managementresponse.OK(c, out(v), "更新成功")
}
func (h *Handler) delete(c *gin.Context) {
	id, e := id(c)
	if e == nil {
		e = h.service.Delete(c, id)
	}
	if e != nil {
		fail(c, e)
		return
	}
	managementresponse.OK(c, nil, "删除成功")
}
