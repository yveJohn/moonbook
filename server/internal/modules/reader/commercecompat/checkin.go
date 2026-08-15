package commercecompat

import (
	"net/http"
	"strconv"

	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/gin-gonic/gin"
)

type checkinResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

func RegisterCheckinRoutes(group *gin.RouterGroup, service commercecontract.CheckinReader, auth *readerauth.Service) {
	handler := checkinHandler{service: service}
	routes := group.Group("/reader/me").Use(readerauth.RequireReader(auth))
	routes.GET("/checkin/status", handler.status)
	routes.POST("/checkin", handler.checkin)
}

type checkinHandler struct {
	service commercecontract.CheckinReader
}

func readerID(ctx *gin.Context) (int64, error) {
	identity, ok := readerauth.ReaderIdentity(ctx)
	if !ok {
		return 0, readerauth.ErrInvalidSession
	}
	return identity.ReaderID, nil
}

func checkinFail(ctx *gin.Context, err error) {
	public := apperror.Expose(err)
	code := 500
	if public.Code == apperror.CodeUnauthenticated {
		code = 401
	}
	ctx.JSON(http.StatusOK, checkinResponse{Code: code, Msg: public.Message})
}

func checkinJSON(value commercecontract.CheckinStatus) map[string]any {
	return map[string]any{
		"todayChecked": value.TodayChecked, "continuousDays": value.ContinuousDays,
		"todayRewardCoin": strconv.FormatInt(value.TodayRewardCoin, 10), "rewardRandom": value.RewardRandom,
		"rewardText": nullableString(value.RewardText), "checkinAvailable": value.CheckinAvailable,
		"unavailableReason": nullableString(value.UnavailableReason),
	}
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (handler checkinHandler) status(ctx *gin.Context) {
	id, err := readerID(ctx)
	if err != nil {
		checkinFail(ctx, err)
		return
	}
	value, err := handler.service.CheckinStatus(ctx, id)
	if err != nil {
		checkinFail(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, checkinResponse{Code: 200, Msg: "查询成功", Data: checkinJSON(value)})
}

func (handler checkinHandler) checkin(ctx *gin.Context) {
	id, err := readerID(ctx)
	if err != nil {
		checkinFail(ctx, err)
		return
	}
	value, err := handler.service.Checkin(ctx, id)
	if err != nil {
		checkinFail(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, checkinResponse{Code: 200, Msg: "签到成功", Data: checkinJSON(value)})
}
