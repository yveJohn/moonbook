package system

import (
	"github.com/gin-gonic/gin"
)

type InitRouter struct{}

func (s *InitRouter) InitInitRouter(Router *gin.RouterGroup) {
	initRouter := Router.Group("init")
	{
		// Moonbook databases are provisioned by versioned migrations and the
		// administrator bootstrap CLI. Keep only the frontend compatibility probe.
		initRouter.POST("checkdb", dbApi.CheckDB)
	}
}
