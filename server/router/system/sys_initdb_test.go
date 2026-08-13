package system

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestInitRouterExposesOnlyDatabaseStatusProbe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	new(InitRouter).InitInitRouter(engine.Group(""))

	routes := make(map[string]struct{})
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	if _, ok := routes["POST /init/checkdb"]; !ok {
		t.Fatal("POST /init/checkdb is not registered")
	}
	if _, ok := routes["POST /init/initdb"]; ok {
		t.Fatal("POST /init/initdb must not be registered")
	}
}
