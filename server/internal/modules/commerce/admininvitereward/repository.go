package admininvitereward

import (
	"context"
	"database/sql"
	"errors"
)

type SQLRepository struct{ DB *sql.DB }

const configSelect = `SELECT id::text,enabled::text,inviter_reward_coin::text,invitee_reward_coin::text,remark,updated_at::text FROM reader_invite_reward_config WHERE id=1`

func scan(row interface{ Scan(...any) error }, v *Config) error {
	return row.Scan(&v.ID, &v.Enabled, &v.InviterRewardCoin, &v.InviteeRewardCoin, &v.Remark, &v.UpdatedAt)
}

func (r SQLRepository) Get(ctx context.Context) (Config, error) {
	var v Config
	err := scan(r.DB.QueryRowContext(ctx, configSelect), &v)
	return v, err
}

func (r SQLRepository) Update(ctx context.Context, in Input) (Config, error) {
	var v Config
	err := scan(r.DB.QueryRowContext(ctx, `UPDATE reader_invite_reward_config SET enabled=$1,inviter_reward_coin=$2,invitee_reward_coin=$3,remark=$4,updated_at=now() WHERE id=1 RETURNING id::text,enabled::text,inviter_reward_coin::text,invitee_reward_coin::text,remark,updated_at::text`, in.Enabled, in.InviterRewardCoin, in.InviteeRewardCoin, in.Remark), &v)
	if err == sql.ErrNoRows {
		return Config{}, errors.New("invite reward config unavailable")
	}
	return v, err
}
