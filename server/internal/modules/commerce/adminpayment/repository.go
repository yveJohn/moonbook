package adminpayment

import (
	"context"
	"database/sql"
	"os"
)

type SQLRepository struct{ DB *sql.DB }

func (r SQLRepository) List(ctx context.Context) ([]Channel, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id,provider,enabled,currency,token,network FROM reader_payment_channels ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	configured := os.Getenv("MOONBOOK_EPUSDT_PID") != "" && os.Getenv("MOONBOOK_EPUSDT_SECRET") != ""
	items := make([]Channel, 0)
	for rows.Next() {
		var v Channel
		if err := rows.Scan(&v.ID, &v.Provider, &v.Enabled, &v.Currency, &v.Token, &v.Network); err != nil {
			return nil, err
		}
		v.Configured = v.Provider != "epusdt" || configured
		items = append(items, v)
	}
	return items, rows.Err()
}

func (r SQLRepository) SetEnabled(ctx context.Context, id int64, enabled bool) (Channel, error) {
	var v Channel
	err := r.DB.QueryRowContext(ctx, `UPDATE reader_payment_channels SET enabled=$1,updated_at=now() WHERE id=$2 RETURNING id,provider,enabled,currency,token,network`, enabled, id).Scan(&v.ID, &v.Provider, &v.Enabled, &v.Currency, &v.Token, &v.Network)
	if err != nil {
		return Channel{}, err
	}
	v.Configured = v.Provider != "epusdt" || (os.Getenv("MOONBOOK_EPUSDT_PID") != "" && os.Getenv("MOONBOOK_EPUSDT_SECRET") != "")
	return v, nil
}
