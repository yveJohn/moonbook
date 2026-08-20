package initialize

import "github.com/gin-gonic/gin"

func bizPluginV1(group ...*gin.RouterGroup) {
	private := group[0]
	public := group[1]
	holder(public, private)
}
