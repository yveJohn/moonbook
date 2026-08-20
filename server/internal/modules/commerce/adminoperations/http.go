package adminoperations

import (
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/managementresponse"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
)

type Handler struct{ service *Service }

func RegisterRoutes(private *gin.RouterGroup, s *Service) {
	h := &Handler{service: s}
	r := private.Group("reader")
	r.GET("checkins", h.checkins)
	r.GET("inviteRewards", h.inviteRewards)
}
func page(c *gin.Context) (int, int) {
	p, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	n, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if p < 1 {
		p = 1
	}
	if n < 1 || n > 100 {
		n = 20
	}
	return p, n
}
func dateArg(c *gin.Context, k string) string { return c.Query(k) }
func validateDate(v string) error {
	if v == "" {
		return nil
	}
	if _, e := time.Parse("2006-01-02", v); e != nil {
		return apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "日期格式必须为 YYYY-MM-DD")
	}
	return nil
}
func validateTime(v string) error {
	if v == "" {
		return nil
	}
	if _, e := time.Parse(time.RFC3339, v); e != nil {
		return apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "时间格式必须为 RFC3339")
	}
	return nil
}
func checkinOut(v Checkin) map[string]any {
	return map[string]any{"id": v.ID, "readerId": v.ReaderID, "username": v.Username, "nickname": v.Nickname, "checkinDate": v.CheckinDate, "continuousDays": v.ContinuousDays, "baseRewardCoin": v.BaseRewardCoin, "milestoneRewardCoin": v.MilestoneRewardCoin, "totalRewardCoin": v.TotalRewardCoin, "idempotencyKey": v.IdempotencyKey, "createdAt": v.CreatedAt}
}
func rewardOut(v InviteReward) map[string]any {
	return map[string]any{"id": v.ID, "relationId": v.RelationID, "inviterReaderId": v.InviterReaderID, "inviterUsername": v.InviterUsername, "inviterNickname": v.InviterNickname, "inviteeReaderId": v.InviteeReaderID, "inviteeUsername": v.InviteeUsername, "inviteeNickname": v.InviteeNickname, "rewardStage": v.RewardStage, "rewardCoin": v.RewardCoin, "status": v.Status, "idempotencyKey": v.IdempotencyKey, "grantedAt": v.GrantedAt, "remark": v.Remark, "createdAt": v.CreatedAt, "updatedAt": v.UpdatedAt}
}
func (h *Handler) checkins(c *gin.Context) {
	p, n := page(c)
	f := CheckinFilter{ReaderKeyword: c.Query("readerKeyword"), StartDate: dateArg(c, "startDate"), EndDate: dateArg(c, "endDate"), Page: p, PageSize: n}
	if e := validateDate(f.StartDate); e != nil {
		apperror.WriteManagement(c, e)
		return
	}
	if e := validateDate(f.EndDate); e != nil {
		apperror.WriteManagement(c, e)
		return
	}
	rows, total, e := h.service.ListCheckins(c, f)
	if e != nil {
		apperror.WriteManagement(c, e)
		return
	}
	out := make([]any, 0, len(rows))
	for _, v := range rows {
		out = append(out, checkinOut(v))
	}
	managementresponse.OK(c, managementresponse.Page{List: out, Total: total, Page: p, PageSize: n}, "获取成功")
}
func (h *Handler) inviteRewards(c *gin.Context) {
	p, n := page(c)
	f := InviteRewardFilter{InviterKeyword: c.Query("inviterKeyword"), InviteeKeyword: c.Query("inviteeKeyword"), RewardStage: c.Query("rewardStage"), Status: c.Query("status"), StartTime: c.Query("startTime"), EndTime: c.Query("endTime"), Page: p, PageSize: n}
	for _, v := range []string{f.StartTime, f.EndTime} {
		if e := validateTime(v); e != nil {
			apperror.WriteManagement(c, e)
			return
		}
	}
	if f.RewardStage != "" && f.RewardStage != "register" && f.RewardStage != "first_recharge" {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "rewardStage 无效"))
		return
	}
	if f.Status != "" && f.Status != "granted" && f.Status != "skipped" && f.Status != "failed" {
		apperror.WriteManagement(c, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "status 无效"))
		return
	}
	rows, total, e := h.service.ListInviteRewards(c, f)
	if e != nil {
		apperror.WriteManagement(c, e)
		return
	}
	out := make([]any, 0, len(rows))
	for _, v := range rows {
		out = append(out, rewardOut(v))
	}
	managementresponse.OK(c, managementresponse.Page{List: out, Total: total, Page: p, PageSize: n}, "获取成功")
}
