package managementresponse

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type envelope struct {
	Code int    `json:"code"`
	Data any    `json:"data"`
	Msg  string `json:"msg"`
}

type Page struct {
	List     any   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

func OK(c *gin.Context, data any, message string) {
	c.JSON(http.StatusOK, envelope{Code: 0, Data: data, Msg: message})
}

func Message(c *gin.Context, message string) {
	OK(c, map[string]any{}, message)
}
