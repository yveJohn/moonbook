package admincheckin

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
)

type SQLRepository struct{ DB *sql.DB }

const selectRule = `SELECT id::text,rule_type,COALESCE(continuous_days::text,''),reward_mode,COALESCE(fixed_coin::text,''),COALESCE(min_coin::text,''),COALESCE(max_coin::text,''),status,sort_order::text,remark FROM reader_checkin_reward_rules`

func scan(row interface{ Scan(...any) error }, v *Rule) error {
	return row.Scan(&v.ID, &v.RuleType, &v.ContinuousDays, &v.RewardMode, &v.FixedCoin, &v.MinCoin, &v.MaxCoin, &v.Status, &v.SortOrder, &v.Remark)
}
func valid(in Input) error {
	if in.RuleType != "daily" && in.RuleType != "continuous" {
		return errors.New("invalid rule type")
	}
	if in.RewardMode != "fixed" && in.RewardMode != "random" {
		return errors.New("invalid reward mode")
	}
	if in.Status != "enabled" && in.Status != "disabled" {
		return errors.New("invalid rule status")
	}
	if in.RuleType == "continuous" {
		n, e := strconv.Atoi(in.ContinuousDays)
		if e != nil || n <= 0 {
			return errors.New("invalid continuous days")
		}
	} else if in.ContinuousDays != "" {
		return errors.New("daily rule cannot set continuous days")
	}
	if in.RewardMode == "fixed" {
		n, e := strconv.ParseInt(in.FixedCoin, 10, 64)
		if e != nil || n <= 0 || in.MinCoin != "" || in.MaxCoin != "" {
			return errors.New("invalid fixed reward")
		}
	} else {
		min, e := strconv.ParseInt(in.MinCoin, 10, 64)
		max, e2 := strconv.ParseInt(in.MaxCoin, 10, 64)
		if e != nil || e2 != nil || min <= 0 || max < min || in.FixedCoin != "" {
			return errors.New("invalid random reward")
		}
	}
	if in.SortOrder != "" {
		n, e := strconv.Atoi(in.SortOrder)
		if e != nil || n < 0 {
			return errors.New("invalid sort order")
		}
	}
	if len([]rune(in.Remark)) > 255 {
		return errors.New("remark too long")
	}
	return nil
}
func (r SQLRepository) List(ctx context.Context, k string, p, n int) ([]Rule, int64, error) {
	where := ` WHERE ($1='' OR rule_type=$1)`
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM reader_checkin_reward_rules`+where, k).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, selectRule+where+` ORDER BY sort_order,id LIMIT $2 OFFSET $3`, k, n, (p-1)*n)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]Rule, 0)
	for rows.Next() {
		var v Rule
		if err := scan(rows, &v); err != nil {
			return nil, 0, err
		}
		items = append(items, v)
	}
	return items, total, rows.Err()
}
func (r SQLRepository) Create(ctx context.Context, in Input) (Rule, error) {
	if err := valid(in); err != nil {
		return Rule{}, err
	}
	return r.mutate(ctx, `INSERT INTO reader_checkin_reward_rules(rule_type,continuous_days,reward_mode,fixed_coin,min_coin,max_coin,status,sort_order,remark) VALUES($1,NULLIF($2,'')::int,$3,NULLIF($4,'')::bigint,NULLIF($5,'')::bigint,NULLIF($6,'')::bigint,$7,COALESCE(NULLIF($8,'')::int,0),$9) RETURNING `+`id::text,rule_type,COALESCE(continuous_days::text,''),reward_mode,COALESCE(fixed_coin::text,''),COALESCE(min_coin::text,''),COALESCE(max_coin::text,''),status,sort_order::text,remark`, in, nil)
}
func (r SQLRepository) Update(ctx context.Context, id int64, in Input) (Rule, error) {
	if id <= 0 {
		return Rule{}, errors.New("invalid id")
	}
	if err := valid(in); err != nil {
		return Rule{}, err
	}
	return r.mutate(ctx, `UPDATE reader_checkin_reward_rules SET rule_type=$1,continuous_days=NULLIF($2,'')::int,reward_mode=$3,fixed_coin=NULLIF($4,'')::bigint,min_coin=NULLIF($5,'')::bigint,max_coin=NULLIF($6,'')::bigint,status=$7,sort_order=COALESCE(NULLIF($8,'')::int,0),remark=$9,updated_at=now() WHERE id=$10 RETURNING `+`id::text,rule_type,COALESCE(continuous_days::text,''),reward_mode,COALESCE(fixed_coin::text,''),COALESCE(min_coin::text,''),COALESCE(max_coin::text,''),status,sort_order::text,remark`, in, id)
}
func (r SQLRepository) mutate(ctx context.Context, q string, in Input, id any) (Rule, error) {
	args := []any{in.RuleType, in.ContinuousDays, in.RewardMode, in.FixedCoin, in.MinCoin, in.MaxCoin, in.Status, in.SortOrder, in.Remark}
	if id != nil {
		args = append(args, id)
	}
	var v Rule
	err := scan(r.DB.QueryRowContext(ctx, q, args...), &v)
	return v, err
}
func (r SQLRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM reader_checkin_reward_rules WHERE id=$1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
