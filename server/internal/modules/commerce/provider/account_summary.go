package provider

import (
	"context"
	"database/sql"

	commercecontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/contract"
)

type AccountSummary struct{ db *sql.DB }

var (
	_ commercecontract.ReaderAccountSummary = (*AccountSummary)(nil)
	_ commercecontract.InviteRewardReader   = (*AccountSummary)(nil)
)

func NewAccountSummary(db *sql.DB) *AccountSummary { return &AccountSummary{db: db} }

func (provider *AccountSummary) Entitlements(ctx context.Context, readerID int64) (commercecontract.EntitlementSummary, error) {
	if provider == nil || provider.db == nil || readerID <= 0 {
		return commercecontract.EntitlementSummary{}, commercecontract.ErrInvalidRequest
	}
	summary := commercecontract.EntitlementSummary{ReaderID: readerID, BookIDs: []int64{}}
	rows, err := provider.db.QueryContext(ctx, `SELECT target_id FROM commerce_entitlements WHERE reader_id=$1 AND entitlement_type='book' AND status='active' AND starts_at<=now() AND (permanent OR expires_at>now()) ORDER BY target_id`, readerID)
	if err != nil {
		return summary, commercecontract.Wrap(commercecontract.ErrUnavailable, err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return summary, commercecontract.Wrap(commercecontract.ErrUnavailable, err)
		}
		summary.BookIDs = append(summary.BookIDs, id)
	}
	if err := rows.Err(); err != nil {
		return summary, commercecontract.Wrap(commercecontract.ErrUnavailable, err)
	}
	var expires sql.NullTime
	if err := provider.db.QueryRowContext(ctx, `SELECT COALESCE(bool_or(permanent),false), max(expires_at) FROM commerce_membership_grants WHERE reader_id=$1 AND status='active' AND starts_at<=now() AND (permanent OR expires_at>now())`, readerID).Scan(&summary.MembershipPermanent, &expires); err != nil {
		return summary, commercecontract.Wrap(commercecontract.ErrUnavailable, err)
	}
	summary.MembershipActive = summary.MembershipPermanent || expires.Valid
	if expires.Valid {
		summary.MembershipExpiresAt = &expires.Time
	}
	return summary, nil
}

func (provider *AccountSummary) MembershipProducts(ctx context.Context) ([]commercecontract.MembershipProduct, error) {
	if provider == nil || provider.db == nil {
		return nil, commercecontract.ErrUnavailable
	}
	rows, err := provider.db.QueryContext(ctx, `SELECT id,product_name,price_coin,allow_bonus_coin,duration_days,sale_status,sort_order,created_at,updated_at FROM commerce_products WHERE product_type='membership' AND sale_status='on_sale' ORDER BY sort_order,id`)
	if err != nil {
		return nil, commercecontract.Wrap(commercecontract.ErrUnavailable, err)
	}
	defer rows.Close()
	products := []commercecontract.MembershipProduct{}
	for rows.Next() {
		var product commercecontract.MembershipProduct
		var days sql.NullInt64
		if err := rows.Scan(&product.ID, &product.Name, &product.PriceCoin, &product.AllowBonusCoin, &days, &product.SaleStatus, &product.SortOrder, &product.CreatedAt, &product.UpdatedAt); err != nil {
			return nil, commercecontract.Wrap(commercecontract.ErrUnavailable, err)
		}
		if days.Valid {
			value := int(days.Int64)
			product.DurationDays = &value
		}
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		return nil, commercecontract.Wrap(commercecontract.ErrUnavailable, err)
	}
	return products, nil
}

func (provider *AccountSummary) InviteRewardSummary(ctx context.Context, readerID int64) (commercecontract.InviteRewardSummary, error) {
	if provider == nil || provider.db == nil || readerID <= 0 {
		return commercecontract.InviteRewardSummary{}, commercecontract.ErrInvalidRequest
	}
	var summary commercecontract.InviteRewardSummary
	if err := provider.db.QueryRowContext(ctx, `SELECT COALESCE((SELECT invitee_reward_coin FROM reader_invite_reward_config WHERE id=1 AND enabled),0),100,COALESCE((SELECT sum(amount) FROM reader_wallet_ledgers WHERE reader_id=$1 AND biz_type IN ('invite_reward','invite_first_recharge_reward') AND coin_type='bonus' AND direction='income'),0)`, readerID).Scan(&summary.RegisterRewardCoin, &summary.FirstRechargeRewardCoin, &summary.TotalRewardCoin); err != nil {
		return summary, commercecontract.Wrap(commercecontract.ErrUnavailable, err)
	}
	return summary, nil
}
