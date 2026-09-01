package account

import (
	"database/sql"
	"net/http"
	"strconv"

	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	readerinvite "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/invite"
	readerwire "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/wire"
	"github.com/gin-gonic/gin"
)

type response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

type Handler struct {
	db      *sql.DB
	summary commercecontract.ReaderAccountSummary
	rewards commercecontract.InviteRewardReader
}

func RegisterRoutes(group *gin.RouterGroup, db *sql.DB, auth *readerauth.Service, summary commercecontract.ReaderAccountSummary, rewards commercecontract.InviteRewardReader) {
	h := &Handler{db: db, summary: summary, rewards: rewards}
	r := group.Group("/reader").Use(readerauth.RequireReader(auth))
	r.GET("/me/entitlements", h.entitlements)
	r.GET("/products/membership", h.membershipProducts)
	r.POST("/me/invite/code", h.inviteDashboard)
}

func (h *Handler) readerID(c *gin.Context) (int64, bool) {
	i, ok := readerauth.ReaderIdentity(c)
	return i.ReaderID, ok
}

func accountError(c *gin.Context, err error) {
	if err != nil {
		c.JSON(http.StatusOK, response{Code: 500, Msg: "个人数据服务暂不可用"})
		return
	}
	c.JSON(http.StatusOK, response{Code: 401, Msg: "认证失败，无法访问系统资源"})
}

func (h *Handler) entitlements(c *gin.Context) {
	rid, ok := h.readerID(c)
	if !ok {
		accountError(c, nil)
		return
	}
	summary, err := h.summary.Entitlements(c, rid)
	if err != nil {
		accountError(c, err)
		return
	}
	books := make([]string, 0, len(summary.BookIDs))
	for _, id := range summary.BookIDs {
		books = append(books, strconv.FormatInt(id, 10))
	}
	out := map[string]any{"readerId": strconv.FormatInt(summary.ReaderID, 10), "bookIds": books, "membershipActive": summary.MembershipActive, "membershipPermanent": summary.MembershipPermanent, "membershipExpireTime": nil}
	if summary.MembershipExpiresAt != nil {
		out["membershipExpireTime"] = readerwire.DateTime(*summary.MembershipExpiresAt)
	}
	c.JSON(http.StatusOK, response{Code: 200, Msg: "查询成功", Data: out})
}

func (h *Handler) membershipProducts(c *gin.Context) {
	products, err := h.summary.MembershipProducts(c)
	if err != nil {
		accountError(c, err)
		return
	}
	out := make([]map[string]any, 0, len(products))
	for _, product := range products {
		var duration any
		if product.DurationDays != nil {
			duration = *product.DurationDays
		}
		out = append(out, map[string]any{"id": strconv.FormatInt(product.ID, 10), "productType": "membership", "targetId": nil, "productName": product.Name, "priceCoin": strconv.FormatInt(product.PriceCoin, 10), "allowBonusCoin": product.AllowBonusCoin, "durationDays": duration, "saleStatus": product.SaleStatus, "sortOrder": product.SortOrder, "remark": "", "createTime": readerwire.DateTime(product.CreatedAt), "updateTime": readerwire.DateTime(product.UpdatedAt)})
	}
	c.JSON(http.StatusOK, response{Code: 200, Msg: "查询成功", Data: out})
}

func (h *Handler) inviteDashboard(c *gin.Context) {
	rid, ok := h.readerID(c)
	if !ok {
		accountError(c, nil)
		return
	}
	tx, err := h.db.BeginTx(c, nil)
	if err != nil {
		accountError(c, err)
		return
	}
	defer tx.Rollback()
	var code string
	err = tx.QueryRowContext(c, `SELECT code FROM reader_invite_codes WHERE inviter_reader_id=$1 FOR UPDATE`, rid).Scan(&code)
	if err == sql.ErrNoRows {
		code, err = readerinvite.GenerateCodeForReader(c, tx, rid)
	}
	if err != nil {
		accountError(c, err)
		return
	}
	if err = tx.Commit(); err != nil {
		accountError(c, err)
		return
	}
	rewards, err := h.rewards.InviteRewardSummary(c, rid)
	if err != nil {
		accountError(c, err)
		return
	}
	var invited int64
	if err := h.db.QueryRowContext(c, `SELECT count(*) FROM reader_invite_relations WHERE inviter_reader_id=$1 AND status='active'`, rid).Scan(&invited); err != nil {
		accountError(c, err)
		return
	}
	rewardItems := make([]map[string]any, 0, len(rewards.Records))
	for _, reward := range rewards.Records {
		rewardItems = append(rewardItems, map[string]any{
			"id":          strconv.FormatInt(reward.ID, 10),
			"rewardStage": reward.RewardStage,
			"rewardCoin":  strconv.FormatInt(reward.RewardCoin, 10),
			"grantTime":   readerwire.DateTimePointer(reward.GrantedAt),
			"remark":      reward.Remark,
		})
	}
	data := map[string]any{"readerId": strconv.FormatInt(rid, 10), "inviteCode": code, "inviteCodeAvailable": true, "shareTextTemplate": "邀请你加入月白书城，点击 {{link}} 注册", "registerRewardCoin": strconv.FormatInt(rewards.RegisterRewardCoin, 10), "firstRechargeRewardCoin": strconv.FormatInt(rewards.FirstRechargeRewardCoin, 10), "invitedCount": strconv.FormatInt(invited, 10), "totalRewardCoin": strconv.FormatInt(rewards.TotalRewardCoin, 10), "rewards": rewardItems}
	c.JSON(http.StatusOK, response{Code: 200, Msg: "查询成功", Data: data})
}
