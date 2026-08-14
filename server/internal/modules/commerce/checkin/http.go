package checkin

import (
	"net/http"
	"strconv"

	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/gin-gonic/gin"
)

type response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

func RegisterRoutes(group *gin.RouterGroup, service *Service, auth *readerauth.Service) {
	h := &Handler{service: service}
	r := group.Group("/reader/me").Use(readerauth.RequireReader(auth))
	r.GET("/checkin/status", h.status)
	r.POST("/checkin", h.checkin)
}

type Handler struct{ service *Service }

func checkinReaderID(c *gin.Context) (int64, error) {
	x, ok := readerauth.ReaderIdentity(c)
	if !ok {
		return 0, readerauth.ErrInvalidSession
	}
	return x.ReaderID, nil
}
func checkinFail(c *gin.Context, e error) {
	p := apperror.Expose(e)
	code := 500
	if p.Code == apperror.CodeUnauthenticated {
		code = 401
	}
	c.JSON(http.StatusOK, response{Code: code, Msg: p.Message})
}
func statusJSON(v Status) map[string]any {
	return map[string]any{"todayChecked": v.TodayChecked, "continuousDays": v.ContinuousDays, "todayRewardCoin": strconv.FormatInt(v.TodayRewardCoin, 10), "rewardRandom": v.RewardRandom, "rewardText": nullString(v.RewardText), "checkinAvailable": v.CheckinAvailable, "unavailableReason": nullString(v.UnavailableReason)}
}
func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
func (h *Handler) status(c *gin.Context) {
	id, e := checkinReaderID(c)
	if e != nil {
		checkinFail(c, e)
		return
	}
	v, e := h.service.Status(c, id)
	if e != nil {
		checkinFail(c, e)
		return
	}
	c.JSON(http.StatusOK, response{Code: 200, Msg: "查询成功", Data: statusJSON(v)})
}
func (h *Handler) checkin(c *gin.Context) {
	id, e := checkinReaderID(c)
	if e != nil {
		checkinFail(c, e)
		return
	}
	v, e := h.service.Checkin(c, id)
	if e != nil {
		checkinFail(c, e)
		return
	}
	c.JSON(http.StatusOK, response{Code: 200, Msg: "签到成功", Data: statusJSON(v)})
}
