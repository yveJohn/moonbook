package checkin

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/wallet"
)

var ErrUnavailable = errors.New("check-in reward is unavailable")

type SQLRepository struct{ DB *sql.DB }
type rewardRule struct {
	Mode            string
	Fixed, Min, Max int64
}

func today() time.Time {
	n := time.Now().UTC()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}
func (r SQLRepository) Status(ctx context.Context, readerID int64) (Status, error) {
	return r.statusAt(ctx, readerID, today())
}
func (r SQLRepository) statusAt(ctx context.Context, readerID int64, day time.Time) (Status, error) {
	var s Status
	var total int64
	err := r.DB.QueryRowContext(ctx, `SELECT continuous_days,total_reward_coin FROM reader_checkin_records WHERE reader_id=$1 AND checkin_date=$2`, readerID, day).Scan(&s.ContinuousDays, &total)
	if err == nil {
		s.TodayChecked = true
		s.TodayRewardCoin = total
		s.CheckinAvailable = true
		return s, nil
	}
	if err != sql.ErrNoRows {
		return s, err
	}
	s.ContinuousDays, err = r.nextDays(ctx, readerID, day)
	if err != nil {
		return s, err
	}
	rule, err := r.rule(ctx, "daily", 0)
	if err != nil {
		return s, err
	}
	milestone, err := r.rule(ctx, "continuous", s.ContinuousDays)
	if err != nil {
		return s, err
	}
	if rule == nil {
		s.CheckinAvailable = false
		s.UnavailableReason = "签到奖励暂未配置"
		return s, nil
	}
	s.CheckinAvailable = true
	s.RewardRandom = rule.Mode == "random" || (milestone != nil && milestone.Mode == "random")
	if s.RewardRandom {
		s.RewardText = "随机金币奖励"
		return s, nil
	}
	s.TodayRewardCoin = rule.Fixed
	if milestone != nil {
		s.TodayRewardCoin += milestone.Fixed
	}
	return s, nil
}
func (r SQLRepository) nextDays(ctx context.Context, readerID int64, day time.Time) (int, error) {
	var n sql.NullInt64
	err := r.DB.QueryRowContext(ctx, `SELECT continuous_days FROM reader_checkin_records WHERE reader_id=$1 AND checkin_date=$2`, readerID, day.AddDate(0, 0, -1)).Scan(&n)
	if err == sql.ErrNoRows || !n.Valid {
		return 1, nil
	}
	return int(n.Int64) + 1, err
}
func (r SQLRepository) rule(ctx context.Context, typ string, days int) (*rewardRule, error) {
	var v rewardRule
	var fixed, min, max sql.NullInt64
	var q string
	if typ == "daily" {
		q = `SELECT reward_mode,fixed_coin,min_coin,max_coin FROM reader_checkin_reward_rules WHERE rule_type='daily' AND status='enabled' ORDER BY sort_order,id DESC LIMIT 1`
	} else {
		q = `SELECT reward_mode,fixed_coin,min_coin,max_coin FROM reader_checkin_reward_rules WHERE rule_type='continuous' AND continuous_days=$1 AND status='enabled' LIMIT 1`
	}
	var err error
	if typ == "daily" {
		err = r.DB.QueryRowContext(ctx, q).Scan(&v.Mode, &fixed, &min, &max)
	} else {
		err = r.DB.QueryRowContext(ctx, q, days).Scan(&v.Mode, &fixed, &min, &max)
	}
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if fixed.Valid {
		v.Fixed = fixed.Int64
	}
	if min.Valid {
		v.Min = min.Int64
	}
	if max.Valid {
		v.Max = max.Int64
	}
	return &v, nil
}
func reward(rule *rewardRule) (int64, error) {
	if rule.Mode == "fixed" {
		return rule.Fixed, nil
	}
	if rule.Mode != "random" || rule.Min <= 0 || rule.Max < rule.Min {
		return 0, ErrUnavailable
	}
	span := new(big.Int).Sub(big.NewInt(rule.Max), big.NewInt(rule.Min))
	span.Add(span, big.NewInt(1))
	n, e := rand.Int(rand.Reader, span)
	if e != nil {
		return 0, e
	}
	return new(big.Int).Add(n, big.NewInt(rule.Min)).Int64(), nil
}
func (r SQLRepository) Checkin(ctx context.Context, readerID int64) (Status, error) {
	day := today()
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return Status{}, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, readerID); err != nil {
		return Status{}, err
	}
	var existing Status
	var total int64
	if err = tx.QueryRowContext(ctx, `SELECT continuous_days,total_reward_coin FROM reader_checkin_records WHERE reader_id=$1 AND checkin_date=$2`, readerID, day).Scan(&existing.ContinuousDays, &total); err == nil {
		existing.TodayChecked = true
		existing.TodayRewardCoin = total
		existing.CheckinAvailable = true
		return existing, nil
	}
	if err != sql.ErrNoRows {
		return Status{}, err
	}
	days := 1
	var yesterday sql.NullInt64
	if err = tx.QueryRowContext(ctx, `SELECT continuous_days FROM reader_checkin_records WHERE reader_id=$1 AND checkin_date=$2`, readerID, day.AddDate(0, 0, -1)).Scan(&yesterday); err == nil && yesterday.Valid {
		days = int(yesterday.Int64) + 1
	}
	daily, err := ruleTx(ctx, tx, "daily", 0)
	if err != nil {
		return Status{}, err
	}
	if daily == nil {
		return Status{}, ErrUnavailable
	}
	milestone, err := ruleTx(ctx, tx, "continuous", days)
	if err != nil {
		return Status{}, err
	}
	base, err := reward(daily)
	if err != nil {
		return Status{}, err
	}
	extra := int64(0)
	if milestone != nil {
		extra, err = reward(milestone)
		if err != nil {
			return Status{}, err
		}
	}
	total = base + extra
	key := fmt.Sprintf("checkin:%d:%s", readerID, day.Format("20060102"))
	var recordID int64
	if err = tx.QueryRowContext(ctx, `INSERT INTO reader_checkin_records(reader_id,checkin_date,continuous_days,base_reward_coin,milestone_reward_coin,total_reward_coin,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, readerID, day, days, base, extra, total, key).Scan(&recordID); err != nil {
		return Status{}, err
	}
	biz := fmt.Sprintf("%d", recordID)
	ledgerNo := fmt.Sprintf("CHK%d", recordID)
	if _, err = wallet.MutateTx(ctx, tx, wallet.Mutation{ReaderID: readerID, BizType: "checkin", BizID: &biz, Direction: "income", CoinType: "bonus", Amount: total, LedgerNo: ledgerNo, Remark: stringPtr("签到奖励"), IdempotencyKey: &key}); err != nil {
		return Status{}, err
	}
	if err = tx.Commit(); err != nil {
		return Status{}, err
	}
	return Status{TodayChecked: true, ContinuousDays: days, TodayRewardCoin: total, CheckinAvailable: true}, nil
}
func ruleTx(ctx context.Context, tx *sql.Tx, typ string, days int) (*rewardRule, error) {
	var v rewardRule
	var fixed, min, max sql.NullInt64
	var err error
	if typ == "daily" {
		err = tx.QueryRowContext(ctx, `SELECT reward_mode,fixed_coin,min_coin,max_coin FROM reader_checkin_reward_rules WHERE rule_type='daily' AND status='enabled' ORDER BY sort_order,id DESC LIMIT 1`).Scan(&v.Mode, &fixed, &min, &max)
	} else {
		err = tx.QueryRowContext(ctx, `SELECT reward_mode,fixed_coin,min_coin,max_coin FROM reader_checkin_reward_rules WHERE rule_type='continuous' AND continuous_days=$1 AND status='enabled'`, days).Scan(&v.Mode, &fixed, &min, &max)
	}
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if fixed.Valid {
		v.Fixed = fixed.Int64
	}
	if min.Valid {
		v.Min = min.Int64
	}
	if max.Valid {
		v.Max = max.Int64
	}
	return &v, nil
}
func stringPtr(v string) *string { return &v }
