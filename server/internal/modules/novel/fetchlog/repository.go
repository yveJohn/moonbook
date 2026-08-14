package fetchlog

import (
	"context"
	"database/sql"
)

type SQLRepository struct{ DB *sql.DB }
type Entry struct {
	TaskID, SourceID, SourceName, BoardID, BoardName, ThreadURL, RequestURL, Stage, Status, TargetBookID, Message, DetailJSON string
	HTTPStatus, ResponseBytes, ElapsedMs, ItemCount                                                                           int
}

const cols = `l.id::text,COALESCE(l.task_id::text,''),COALESCE(l.source_id::text,''),COALESCE(l.source_name,''),COALESCE(l.board_id::text,''),COALESCE(l.board_name,''),COALESCE(l.thread_url,''),COALESCE(l.request_url,''),l.stage,l.status,COALESCE(l.http_status::text,''),COALESCE(l.response_bytes::text,''),COALESCE(l.elapsed_ms::text,''),COALESCE(l.item_count::text,''),COALESCE(l.target_book_id::text,''),COALESCE(l.message,''),COALESCE(l.detail_json::text,''),l.created_at::text`

func scan(row interface{ Scan(...any) error }, v *Log) error {
	return row.Scan(&v.ID, &v.TaskID, &v.SourceID, &v.SourceName, &v.BoardID, &v.BoardName, &v.ThreadURL, &v.RequestURL, &v.Stage, &v.Status, &v.HTTPStatus, &v.ResponseBytes, &v.ElapsedMs, &v.ItemCount, &v.TargetBookID, &v.Message, &v.DetailJSON, &v.CreatedAt)
}
func (r SQLRepository) List(ctx context.Context, keyword, status, stage, taskID string, page, size int) ([]Log, int64, error) {
	where := ` WHERE ($1='' OR l.request_url ILIKE '%'||$1||'%' OR l.message ILIKE '%'||$1||'%') AND ($2='' OR l.status=$2) AND ($3='' OR l.stage=$3) AND ($4='' OR l.task_id::text=$4)`
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM novel_crawl_fetch_log l`+where, keyword, status, stage, taskID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT `+cols+` FROM novel_crawl_fetch_log l`+where+` ORDER BY l.created_at DESC,l.id DESC LIMIT $5 OFFSET $6`, keyword, status, stage, taskID, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Log{}
	for rows.Next() {
		var v Log
		if err := scan(rows, &v); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}
func (r SQLRepository) Get(ctx context.Context, id int64) (Log, error) {
	var v Log
	err := scan(r.DB.QueryRowContext(ctx, `SELECT `+cols+` FROM novel_crawl_fetch_log l WHERE l.id=$1`, id), &v)
	return v, err
}
func (r SQLRepository) Append(ctx context.Context, e Entry) error {
	_, err := r.DB.ExecContext(ctx, `INSERT INTO novel_crawl_fetch_log(task_id,source_id,source_name,board_id,board_name,thread_url,request_url,stage,status,http_status,response_bytes,elapsed_ms,item_count,target_book_id,message,detail_json) VALUES(NULLIF($1,'')::bigint,NULLIF($2,'')::bigint,$3,NULLIF($4,'')::bigint,$5,$6,$7,$8,$9,NULLIF($10,0),NULLIF($11,0),NULLIF($12,0),NULLIF($13,0),NULLIF($14,'')::bigint,NULLIF($15,''),NULLIF($16,'')::jsonb)`, e.TaskID, e.SourceID, e.SourceName, e.BoardID, e.BoardName, e.ThreadURL, e.RequestURL, e.Stage, e.Status, e.HTTPStatus, e.ResponseBytes, e.ElapsedMs, e.ItemCount, e.TargetBookID, e.Message, e.DetailJSON)
	return err
}
