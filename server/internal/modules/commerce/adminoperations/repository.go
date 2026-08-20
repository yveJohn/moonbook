package adminoperations

import (
	"context"
	"database/sql"
)

type SQLRepository struct{ DB *sql.DB }

func (r SQLRepository) ListCheckins(ctx context.Context, f CheckinFilter) ([]Checkin, int64, error) {
	where := " WHERE ($1='' OR lower(COALESCE(p.username,'')) LIKE lower('%' || $1 || '%') OR lower(COALESCE(p.nickname,'')) LIKE lower('%' || $1 || '%')) AND ($2='' OR c.checkin_date >= $2::date) AND ($3='' OR c.checkin_date <= $3::date)"
	var total int64
	if err := r.DB.QueryRowContext(ctx, "SELECT count(*) FROM reader_checkin_records c LEFT JOIN commerce_reader_search_projection p ON p.reader_id=c.reader_id"+where, f.ReaderKeyword, f.StartDate, f.EndDate).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, "SELECT c.id::text,c.reader_id::text,COALESCE(p.username,''),COALESCE(p.nickname,''),c.checkin_date::text,c.continuous_days::text,c.base_reward_coin::text,c.milestone_reward_coin::text,c.total_reward_coin::text,c.idempotency_key,c.created_at::text FROM reader_checkin_records c LEFT JOIN commerce_reader_search_projection p ON p.reader_id=c.reader_id"+where+" ORDER BY c.checkin_date DESC,c.id DESC LIMIT $4 OFFSET $5", f.ReaderKeyword, f.StartDate, f.EndDate, f.PageSize, (f.Page-1)*f.PageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Checkin, 0)
	for rows.Next() {
		var v Checkin
		if err := rows.Scan(&v.ID, &v.ReaderID, &v.Username, &v.Nickname, &v.CheckinDate, &v.ContinuousDays, &v.BaseRewardCoin, &v.MilestoneRewardCoin, &v.TotalRewardCoin, &v.IdempotencyKey, &v.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}
func (r SQLRepository) ListInviteRewards(ctx context.Context, f InviteRewardFilter) ([]InviteReward, int64, error) {
	where := " WHERE ($1='' OR lower(COALESCE(ip.username,'')) LIKE lower('%' || $1 || '%') OR lower(COALESCE(ip.nickname,'')) LIKE lower('%' || $1 || '%')) AND ($2='' OR lower(COALESCE(ep.username,'')) LIKE lower('%' || $2 || '%') OR lower(COALESCE(ep.nickname,'')) LIKE lower('%' || $2 || '%')) AND ($3='' OR x.reward_stage=$3) AND ($4='' OR x.status=$4) AND ($5='' OR x.granted_at >= $5::timestamptz) AND ($6='' OR x.granted_at <= $6::timestamptz)"
	base := " FROM reader_invite_reward_records x LEFT JOIN commerce_reader_search_projection ip ON ip.reader_id=x.inviter_reader_id LEFT JOIN commerce_reader_search_projection ep ON ep.reader_id=x.invitee_reader_id"
	var total int64
	if err := r.DB.QueryRowContext(ctx, "SELECT count(*)"+base+where, f.InviterKeyword, f.InviteeKeyword, f.RewardStage, f.Status, f.StartTime, f.EndTime).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, "SELECT x.id::text,x.relation_id::text,x.inviter_reader_id::text,COALESCE(ip.username,''),COALESCE(ip.nickname,''),x.invitee_reader_id::text,COALESCE(ep.username,''),COALESCE(ep.nickname,''),x.reward_stage,x.reward_coin::text,x.status,x.idempotency_key,COALESCE(x.granted_at::text,''),x.remark,x.created_at::text,x.updated_at::text"+base+where+" ORDER BY x.granted_at DESC NULLS LAST,x.id DESC LIMIT $7 OFFSET $8", f.InviterKeyword, f.InviteeKeyword, f.RewardStage, f.Status, f.StartTime, f.EndTime, f.PageSize, (f.Page-1)*f.PageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]InviteReward, 0)
	for rows.Next() {
		var v InviteReward
		if err := rows.Scan(&v.ID, &v.RelationID, &v.InviterReaderID, &v.InviterUsername, &v.InviterNickname, &v.InviteeReaderID, &v.InviteeUsername, &v.InviteeNickname, &v.RewardStage, &v.RewardCoin, &v.Status, &v.IdempotencyKey, &v.GrantedAt, &v.Remark, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}
