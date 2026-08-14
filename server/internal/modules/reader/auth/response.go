package auth

import (
	"net/http"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/gin-gonic/gin"
)

const invalidTokenMessage = "认证失败，无法访问系统资源"

type readerResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

func readerOK(c *gin.Context, data any, message string) {
	c.JSON(http.StatusOK, readerResponse{Code: 200, Msg: message, Data: data})
}

func readerError(c *gin.Context, err error) {
	public := apperror.Expose(err)
	code := 500
	if public.Code == apperror.CodeUnauthenticated {
		code = 401
	}
	if public.Code == apperror.CodeRateLimited {
		code = 429
	}
	c.JSON(http.StatusOK, readerResponse{Code: code, Msg: public.Message})
}

func readerInvalidToken(c *gin.Context) {
	c.JSON(http.StatusOK, readerResponse{Code: 401, Msg: invalidTokenMessage})
}
