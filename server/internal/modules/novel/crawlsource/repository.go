package crawlsource

import (
	"context"
	"database/sql"
	"errors"
)

type SQLRepository struct{ DB *sql.DB }

const sourceSelect = `SELECT id::text,source_name,base_url,request_charset,cookie_secret_ref,(cookie_secret_ref <> ''),COALESCE(user_agent,''),request_interval_ms::text,enabled,sort_order::text,COALESCE(remark,''),created_at::text,updated_at::text FROM novel_crawl_forum_source`
const sourceReturning = `id::text,source_name,base_url,request_charset,cookie_secret_ref,(cookie_secret_ref <> ''),COALESCE(user_agent,''),request_interval_ms::text,enabled,sort_order::text,COALESCE(remark,''),created_at::text,updated_at::text`

func scan(row interface{ Scan(...any) error }, v *Source) error {
	return row.Scan(&v.ID, &v.SourceName, &v.BaseURL, &v.RequestCharset, &v.CookieSecretRef, &v.CookieConfigured, &v.UserAgent, &v.RequestIntervalMs, &v.Enabled, &v.SortOrder, &v.Remark, &v.CreatedAt, &v.UpdatedAt)
}
func (r SQLRepository) List(ctx context.Context, keyword, enabled string, page, size int) ([]Source, int64, error) {
	where := ` WHERE ($1='' OR source_name ILIKE '%'||$1||'%' OR base_url ILIKE '%'||$1||'%') AND ($2='' OR enabled=($2='true'))`
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM novel_crawl_forum_source`+where, keyword, enabled).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, sourceSelect+where+` ORDER BY sort_order,id LIMIT $3 OFFSET $4`, keyword, enabled, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]Source, 0)
	for rows.Next() {
		var v Source
		if err := scan(rows, &v); err != nil {
			return nil, 0, err
		}
		items = append(items, v)
	}
	return items, total, rows.Err()
}
func (r SQLRepository) Get(ctx context.Context, id int64) (Source, error) {
	var v Source
	err := scan(r.DB.QueryRowContext(ctx, sourceSelect+` WHERE id=$1`, id), &v)
	return v, err
}
func (r SQLRepository) Create(ctx context.Context, in Input) (Source, error) {
	var v Source
	err := scan(r.DB.QueryRowContext(ctx, `INSERT INTO novel_crawl_forum_source(source_name,base_url,request_charset,cookie_secret_ref,user_agent,request_interval_ms,enabled,sort_order,remark) VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9) RETURNING `+sourceReturning, in.SourceName, in.BaseURL, in.RequestCharset, in.CookieSecretRef, in.UserAgent, in.RequestIntervalMs, in.Enabled, in.SortOrder, in.Remark), &v)
	return v, err
}
func (r SQLRepository) Update(ctx context.Context, id int64, in Input) (Source, error) {
	var v Source
	err := scan(r.DB.QueryRowContext(ctx, `UPDATE novel_crawl_forum_source SET source_name=$1,base_url=$2,request_charset=$3,cookie_secret_ref=$4,user_agent=NULLIF($5,''),request_interval_ms=$6,enabled=$7,sort_order=$8,remark=$9,updated_at=now() WHERE id=$10 RETURNING `+sourceReturning, in.SourceName, in.BaseURL, in.RequestCharset, in.CookieSecretRef, in.UserAgent, in.RequestIntervalMs, in.Enabled, in.SortOrder, in.Remark, id), &v)
	return v, err
}
func (r SQLRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM novel_crawl_forum_source WHERE id=$1 AND NOT EXISTS(SELECT 1 FROM novel_crawl_forum_board WHERE source_id=$1)`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("source not found or referenced by board")
	}
	return nil
}
