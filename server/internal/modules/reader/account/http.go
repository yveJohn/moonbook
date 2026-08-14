package account

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

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

type Handler struct{ db *sql.DB }

func RegisterRoutes(group *gin.RouterGroup, db *sql.DB, auth *readerauth.Service) {
	h := &Handler{db: db}
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
	out := map[string]any{"readerId": strconv.FormatInt(rid, 10), "bookIds": []string{}, "membershipActive": false, "membershipPermanent": false, "membershipExpireTime": nil}
	rows, err := h.db.QueryContext(c, `SELECT target_id FROM commerce_entitlements WHERE reader_id=$1 AND entitlement_type='book' AND status='active' AND starts_at<=now() AND (permanent OR expires_at>now()) ORDER BY target_id`, rid)
	if err != nil {
		accountError(c, err)
		return
	}
	defer rows.Close()
	books := make([]string, 0)
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			accountError(c, err)
			return
		}
		books = append(books, strconv.FormatInt(id, 10))
	}
	if err = rows.Err(); err != nil {
		accountError(c, err)
		return
	}
	out["bookIds"] = books
	var permanent bool
	var expire sql.NullTime
	err = h.db.QueryRowContext(c, `SELECT COALESCE(bool_or(permanent),false), max(expires_at) FROM commerce_membership_grants WHERE reader_id=$1 AND status='active' AND starts_at<=now() AND (permanent OR expires_at>now())`, rid).Scan(&permanent, &expire)
	if err != nil {
		accountError(c, err)
		return
	}
	out["membershipPermanent"] = permanent
	out["membershipActive"] = permanent || expire.Valid
	if expire.Valid {
		out["membershipExpireTime"] = readerwire.DateTime(expire.Time)
	}
	c.JSON(http.StatusOK, response{Code: 200, Msg: "查询成功", Data: out})
}

func (h *Handler) membershipProducts(c *gin.Context) {
	rows, err := h.db.QueryContext(c, `SELECT id,product_type,COALESCE(target_id,0),product_name,price_coin,allow_bonus_coin,duration_days,sale_status,sort_order,created_at,updated_at FROM commerce_products WHERE product_type='membership' AND sale_status='on_sale' ORDER BY sort_order,id`)
	if err != nil {
		accountError(c, err)
		return
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, target, price int64
		var typ, name, status string
		var bonus bool
		var days sql.NullInt64
		var sort int
		var created, updated time.Time
		if err = rows.Scan(&id, &typ, &target, &name, &price, &bonus, &days, &status, &sort, &created, &updated); err != nil {
			accountError(c, err)
			return
		}
		var duration any
		if days.Valid {
			duration = int(days.Int64)
		}
		out = append(out, map[string]any{"id": strconv.FormatInt(id, 10), "productType": typ, "targetId": nil, "productName": name, "priceCoin": strconv.FormatInt(price, 10), "allowBonusCoin": bonus, "durationDays": duration, "saleStatus": status, "sortOrder": sort, "remark": "", "createTime": readerwire.DateTime(created), "updateTime": readerwire.DateTime(updated)})
	}
	if err = rows.Err(); err != nil {
		accountError(c, err)
		return
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
	var registerReward, firstRecharge int64
	_ = h.db.QueryRowContext(c, `SELECT invitee_reward_coin,0 FROM reader_invite_reward_config WHERE id=1 AND enabled`).Scan(&registerReward, &firstRecharge)
	var invited int64
	_ = h.db.QueryRowContext(c, `SELECT count(*) FROM reader_invite_relations WHERE inviter_reader_id=$1 AND status='active'`, rid).Scan(&invited)
	var total int64
	_ = h.db.QueryRowContext(c, `SELECT COALESCE(sum(amount),0) FROM reader_wallet_ledgers WHERE reader_id=$1 AND biz_type='invite_reward' AND coin_type='bonus' AND direction='income'`, rid).Scan(&total)
	data := map[string]any{"readerId": strconv.FormatInt(rid, 10), "inviteCode": code, "inviteCodeAvailable": true, "shareTextTemplate": "邀请你加入月白书城，点击 {{link}} 注册", "registerRewardCoin": strconv.FormatInt(registerReward, 10), "firstRechargeRewardCoin": "0", "invitedCount": strconv.FormatInt(invited, 10), "totalRewardCoin": strconv.FormatInt(total, 10), "rewards": []any{}}
	c.JSON(http.StatusOK, response{Code: 200, Msg: "查询成功", Data: data})
}
