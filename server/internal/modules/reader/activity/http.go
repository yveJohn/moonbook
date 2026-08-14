package activity

import (
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/gin-gonic/gin"
	"net/http"
)

func RegisterRoutes(private *gin.RouterGroup, service *Service) {
	private.GET("dashboard/overview", func(c *gin.Context) {
		overview, err := service.Overview(c.Request.Context())
		if err != nil {
			apperror.WriteManagement(c, apperror.New(apperror.CodeInternal, http.StatusInternalServerError, "运营数据查询失败"))
			return
		}
		managementresponse.OK(c, overview, "获取成功")
	})
}
