package adminproduct

import (
	"net/http"
	"strconv"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/flipped-aurora/gin-vue-admin/server/middleware"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *Service }

type productRequest struct {
	ProductType    string `json:"productType" binding:"required"`
	TargetID       string `json:"targetId"`
	ProductName    string `json:"productName" binding:"required"`
	PriceCoin      string `json:"priceCoin" binding:"required"`
	AllowBonusCoin bool   `json:"allowBonusCoin"`
	DurationDays   *int   `json:"durationDays"`
	SaleStatus     string `json:"saleStatus" binding:"required"`
	SortOrder      int    `json:"sortOrder"`
}

func RegisterRoutes(private *gin.RouterGroup, service *Service) {
	h := &Handler{service: service}
	read := private.Group("reader")
	write := private.Group("reader").Use(middleware.OperationRecord())
	read.GET("products", h.list)
	write.POST("products", h.create)
	write.PUT("products/:id", h.update)
	write.DELETE("products/:id", h.delete)
}

func out(p Product) map[string]any {
	return map[string]any{"id": p.ID, "productType": p.ProductType, "targetId": p.TargetID, "productName": p.ProductName, "priceCoin": p.PriceCoin, "allowBonusCoin": p.AllowBonusCoin, "durationDays": p.DurationDays, "saleStatus": p.SaleStatus, "sortOrder": p.SortOrder, "sourceType": p.SourceType, "sourceRef": p.SourceRef}
}
func (h *Handler) list(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	items, total, err := h.service.List(c, c.Query("keyword"), c.Query("productType"), c.Query("saleStatus"), page, size)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	rows := make([]any, 0, len(items))
	for _, p := range items {
		rows = append(rows, out(p))
	}
	managementresponse.OK(c, managementresponse.Page{List: rows, Total: total, Page: page, PageSize: size}, "获取成功")
}
func parseID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串")
	}
	return id, nil
}
func readInput(c *gin.Context) (Input, error) {
	var req productRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return Input{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "请求参数无效")
	}
	return Input{ProductType: req.ProductType, TargetID: req.TargetID, ProductName: req.ProductName, PriceCoin: req.PriceCoin, AllowBonusCoin: req.AllowBonusCoin, DurationDays: req.DurationDays, SaleStatus: req.SaleStatus, SortOrder: req.SortOrder}, nil
}
func (h *Handler) create(c *gin.Context) {
	in, err := readInput(c)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	p, err := h.service.Create(c, in)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, out(p), "创建成功")
}
func (h *Handler) update(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	in, err := readInput(c)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	p, err := h.service.Update(c, id, in)
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, out(p), "更新成功")
}
func (h *Handler) delete(c *gin.Context) {
	id, err := parseID(c)
	if err == nil {
		err = h.service.Delete(c, id)
	}
	if err != nil {
		apperror.WriteManagement(c, err)
		return
	}
	managementresponse.OK(c, nil, "删除成功")
}
