package crawlboard

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
)

type SQLRepository struct{ DB *sql.DB }

const boardSelect = `SELECT b.id::text,b.source_id::text,b.source_name,b.board_name,b.board_url,COALESCE(b.board_url_template,''),COALESCE(b.last_cursor,''),b.auto_follow_enabled,b.follow_interval_minutes::text,b.follow_import_limit::text,b.enabled,b.sort_order::text,COALESCE(b.remark,''),b.created_at::text,b.updated_at::text FROM novel_crawl_forum_board b`
const boardReturning = `id::text,source_id::text,source_name,board_name,board_url,COALESCE(board_url_template,''),COALESCE(last_cursor,''),auto_follow_enabled,follow_interval_minutes::text,follow_import_limit::text,enabled,sort_order::text,COALESCE(remark,''),created_at::text,updated_at::text`

func scan(row interface{ Scan(...any) error }, v *Board) error {
	return row.Scan(&v.ID, &v.SourceID, &v.SourceName, &v.BoardName, &v.BoardURL, &v.BoardURLTemplate, &v.LastCursor, &v.AutoFollowEnabled, &v.FollowIntervalMinutes, &v.FollowImportLimit, &v.Enabled, &v.SortOrder, &v.Remark, &v.CreatedAt, &v.UpdatedAt)
}
func (r SQLRepository) List(ctx context.Context, keyword, sourceID, enabled string, page, size int) ([]Board, int64, error) {
	where := ` WHERE ($1='' OR b.board_name ILIKE '%'||$1||'%' OR b.source_name ILIKE '%'||$1||'%') AND ($2='' OR b.source_id::text=$2) AND ($3='' OR b.enabled=($3='true'))`
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM novel_crawl_forum_board b`+where, keyword, sourceID, enabled).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, boardSelect+where+` ORDER BY b.sort_order,b.id LIMIT $4 OFFSET $5`, keyword, sourceID, enabled, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Board{}
	for rows.Next() {
		var v Board
		if err := scan(rows, &v); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}
func (r SQLRepository) source(ctx context.Context, id string) (string, string, bool, error) {
	var name, base string
	var enabled bool
	err := r.DB.QueryRowContext(ctx, `SELECT source_name,base_url,enabled FROM novel_crawl_forum_source WHERE id=$1`, id).Scan(&name, &base, &enabled)
	return name, base, enabled, err
}
func checkURLDomain(raw, base string) bool {
	a, e1 := url.Parse(raw)
	b, e2 := url.Parse(base)
	return e1 == nil && e2 == nil && a.Hostname() == b.Hostname()
}
func (r SQLRepository) Create(ctx context.Context, in Input) (Board, error) {
	name, base, enabled, err := r.source(ctx, in.SourceID)
	if err != nil {
		return Board{}, err
	}
	if in.Enabled && !enabled {
		return Board{}, errors.New("cannot enable board for disabled source")
	}
	if !checkURLDomain(in.BoardURL, base) || (in.BoardURLTemplate != "" && !checkURLDomain(in.BoardURLTemplate, base)) {
		return Board{}, errors.New("board url host differs from source")
	}
	var v Board
	err = scan(r.DB.QueryRowContext(ctx, `INSERT INTO novel_crawl_forum_board(source_id,source_name,board_name,board_url,board_url_template,auto_follow_enabled,follow_interval_minutes,follow_import_limit,enabled,sort_order,remark) VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,$10,$11) RETURNING `+boardReturning, in.SourceID, name, in.BoardName, in.BoardURL, in.BoardURLTemplate, in.AutoFollowEnabled, in.FollowIntervalMinutes, in.FollowImportLimit, in.Enabled, in.SortOrder, in.Remark), &v)
	return v, err
}
func (r SQLRepository) Update(ctx context.Context, id int64, in Input) (Board, error) {
	name, base, enabled, err := r.source(ctx, in.SourceID)
	if err != nil {
		return Board{}, err
	}
	if in.Enabled && !enabled {
		return Board{}, errors.New("cannot enable board for disabled source")
	}
	if !checkURLDomain(in.BoardURL, base) || (in.BoardURLTemplate != "" && !checkURLDomain(in.BoardURLTemplate, base)) {
		return Board{}, errors.New("board url host differs from source")
	}
	var v Board
	err = scan(r.DB.QueryRowContext(ctx, `UPDATE novel_crawl_forum_board b SET source_id=$1,source_name=$2,board_name=$3,board_url=$4,board_url_template=NULLIF($5,''),auto_follow_enabled=$6,follow_interval_minutes=$7,follow_import_limit=$8,enabled=$9,sort_order=$10,remark=$11,updated_at=now() WHERE b.id=$12 RETURNING `+boardReturning, in.SourceID, name, in.BoardName, in.BoardURL, in.BoardURLTemplate, in.AutoFollowEnabled, in.FollowIntervalMinutes, in.FollowImportLimit, in.Enabled, in.SortOrder, in.Remark, id), &v)
	return v, err
}
func (r SQLRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM novel_crawl_forum_board WHERE id=$1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("board not found")
	}
	return nil
}
