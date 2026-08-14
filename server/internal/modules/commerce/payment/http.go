package payment

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(group *gin.RouterGroup, service *Service) {
	group.POST("/reader/payment/epusdt/notify", func(c *gin.Context) { handle(c, service) })
}
func handle(c *gin.Context, s *Service) {
	body, e := io.ReadAll(io.LimitReader(c.Request.Body, 16385))
	if e != nil || len(body) == 0 || len(body) > 16384 {
		c.String(http.StatusBadRequest, "fail")
		return
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(body, &raw) != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}
	fields := make(map[string]string, len(raw))
	for k, v := range raw {
		var text string
		if len(v) == 0 || string(v) == "null" || json.Unmarshal(v, &text) != nil {
			var number json.Number
			if json.Unmarshal(v, &number) != nil {
				c.String(http.StatusBadRequest, "fail")
				return
			}
			text = number.String()
		}
		fields[k] = text
	}
	for _, name := range []string{"pid", "trade_id", "order_id", "amount", "actual_amount", "receive_address", "token", "block_transaction_id", "status", "signature"} {
		if strings.TrimSpace(fields[name]) == "" {
			c.String(http.StatusBadRequest, "fail")
			return
		}
	}
	if fields["pid"] != s.PID || !Verify(fields, fields["signature"], s.Secret) {
		c.String(http.StatusUnauthorized, "fail")
		return
	}
	status := 0
	_, _ = fmt.Sscanf(fields["status"], "%d", &status)
	cb := Callback{PID: fields["pid"], TradeID: fields["trade_id"], OrderNo: fields["order_id"], Amount: fields["amount"], ActualAmount: fields["actual_amount"], ReceiveAddress: fields["receive_address"], Token: fields["token"], TransactionID: fields["block_transaction_id"], Signature: fields["signature"], Status: status, Fields: fields}
	if e = s.Process(c, cb); e != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}
	c.String(http.StatusOK, "success")
}
