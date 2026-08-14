package candidate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type SQLRepository struct{ DB *sql.DB }

const selectColumns = `id::text,source_id::text,source_name,board_id::text,board_name,forum_thread_id,thread_title,COALESCE(display_title,''),thread_url,COALESCE(author_id,''),status,COALESCE(import_task_id::text,''),COALESCE(target_book_id::text,''),COALESCE(last_import_page_no::text,''),COALESCE(last_import_floor_id,''),follow_count::text,COALESCE(last_follow_time::text,''),COALESCE(follow_fail_reason,''),discover_time::text,COALESCE(remark,''),created_at::text,updated_at::text`

func scan(row interface{ Scan(...any) error }, v *Candidate) error {
	return row.Scan(&v.ID, &v.SourceID, &v.SourceName, &v.BoardID, &v.BoardName, &v.ForumThreadID, &v.ThreadTitle, &v.DisplayTitle, &v.ThreadURL, &v.AuthorID, &v.Status, &v.ImportTaskID, &v.TargetBookID, &v.LastImportPageNo, &v.LastImportFloorID, &v.FollowCount, &v.LastFollowTime, &v.FollowFailReason, &v.DiscoverTime, &v.Remark, &v.CreatedAt, &v.UpdatedAt)
}
func (r SQLRepository) List(ctx context.Context, keyword, sourceID, boardID, status string, page, size int) ([]Candidate, int64, error) {
	where := ` WHERE ($1='' OR thread_title ILIKE '%'||$1||'%' OR display_title ILIKE '%'||$1||'%' OR forum_thread_id ILIKE '%'||$1||'%') AND ($2='' OR source_id::text=$2) AND ($3='' OR board_id::text=$3) AND ($4='' OR status=$4)`
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM novel_crawl_thread_candidate`+where, keyword, sourceID, boardID, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT `+selectColumns+` FROM novel_crawl_thread_candidate`+where+` ORDER BY discover_time DESC,id DESC LIMIT $5 OFFSET $6`, keyword, sourceID, boardID, status, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Candidate{}
	for rows.Next() {
		var v Candidate
		if err := scan(rows, &v); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}
func (r SQLRepository) Get(ctx context.Context, id int64) (Candidate, error) {
	var v Candidate
	err := scan(r.DB.QueryRowContext(ctx, `SELECT `+selectColumns+` FROM novel_crawl_thread_candidate WHERE id=$1`, id), &v)
	return v, err
}
func placeholders(n int) string {
	p := make([]string, n)
	for i := range p {
		p[i] = fmt.Sprintf("$%d", i+1)
	}
	return strings.Join(p, ",")
}
func (r SQLRepository) Transition(ctx context.Context, ids []int64, status, requestID string) error {
	if status != "pending" && status != "skipped" {
		return errors.New("invalid candidate status")
	}
	args := make([]any, len(ids)+1)
	for i, id := range ids {
		args[i] = id
	}
	args[len(ids)] = status
	res, err := r.DB.ExecContext(ctx, `UPDATE novel_crawl_thread_candidate SET status=$`+fmt.Sprint(len(ids)+1)+`,updated_at=now() WHERE id IN (`+placeholders(len(ids))+`) AND status IN ('pending','skipped')`, args...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("no candidates can transition")
	}
	return nil
}
func (r SQLRepository) Delete(ctx context.Context, ids []int64) error {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	res, err := r.DB.ExecContext(ctx, `DELETE FROM novel_crawl_thread_candidate WHERE id IN (`+placeholders(len(ids))+`) AND status IN ('pending','skipped')`, args...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("no candidates can be deleted")
	}
	return nil
}
