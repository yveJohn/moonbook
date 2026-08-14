package importtask

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type SQLRepository struct{ DB *sql.DB }

const cols = `t.id::text,t.candidate_id::text,t.source_id::text,t.source_name,t.board_id::text,t.board_name,t.forum_thread_id,t.thread_title,COALESCE(t.display_title,''),t.thread_url,COALESCE(t.target_book_id::text,''),t.import_mode,t.merge_strategy,t.status,t.quality_status,t.total_chapter_count::text,t.imported_chapter_count::text,t.empty_chapter_count::text,t.duplicate_chapter_count::text,t.attempt_count::text,t.max_attempts::text,COALESCE(t.quality_summary,''),COALESCE(t.fail_reason,''),COALESCE(t.operator_name,''),COALESCE(t.start_time::text,''),COALESCE(t.end_time::text,''),t.created_at::text,t.updated_at::text`
const returningCols = `id::text,candidate_id::text,source_id::text,source_name,board_id::text,board_name,forum_thread_id,thread_title,COALESCE(display_title,''),thread_url,COALESCE(target_book_id::text,''),import_mode,merge_strategy,status,quality_status,total_chapter_count::text,imported_chapter_count::text,empty_chapter_count::text,duplicate_chapter_count::text,attempt_count::text,max_attempts::text,COALESCE(quality_summary,''),COALESCE(fail_reason,''),COALESCE(operator_name,''),COALESCE(start_time::text,''),COALESCE(end_time::text,''),created_at::text,updated_at::text`

func scan(row interface{ Scan(...any) error }, v *Task) error {
	return row.Scan(&v.ID, &v.CandidateID, &v.SourceID, &v.SourceName, &v.BoardID, &v.BoardName, &v.ForumThreadID, &v.ThreadTitle, &v.DisplayTitle, &v.ThreadURL, &v.TargetBookID, &v.ImportMode, &v.MergeStrategy, &v.Status, &v.QualityStatus, &v.TotalChapterCount, &v.ImportedChapterCount, &v.EmptyChapterCount, &v.DuplicateChapterCount, &v.AttemptCount, &v.MaxAttempts, &v.QualitySummary, &v.FailReason, &v.OperatorName, &v.StartTime, &v.EndTime, &v.CreatedAt, &v.UpdatedAt)
}
func (r SQLRepository) List(ctx context.Context, keyword, status, sourceID, boardID string, page, size int) ([]Task, int64, error) {
	where := ` WHERE ($1='' OR t.thread_title ILIKE '%'||$1||'%' OR t.display_title ILIKE '%'||$1||'%' OR t.forum_thread_id ILIKE '%'||$1||'%') AND ($2='' OR t.status=$2) AND ($3='' OR t.source_id::text=$3) AND ($4='' OR t.board_id::text=$4)`
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM novel_crawl_import_task t`+where, keyword, status, sourceID, boardID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT `+cols+` FROM novel_crawl_import_task t`+where+` ORDER BY t.created_at DESC,t.id DESC LIMIT $5 OFFSET $6`, keyword, status, sourceID, boardID, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		var v Task
		if err := scan(rows, &v); err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}
func (r SQLRepository) Get(ctx context.Context, id int64) (Task, error) {
	var v Task
	err := scan(r.DB.QueryRowContext(ctx, `SELECT `+cols+` FROM novel_crawl_import_task t WHERE t.id=$1`, id), &v)
	return v, err
}
func (r SQLRepository) Create(ctx context.Context, in CreateInput) (Task, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback()
	var sourceID, sourceName, boardID, boardName, threadID, title, threadURL string
	var targetBook sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT source_id::text,source_name,board_id::text,board_name,forum_thread_id,thread_title,thread_url,target_book_id FROM novel_crawl_thread_candidate WHERE id=$1 FOR UPDATE`, in.CandidateID).Scan(&sourceID, &sourceName, &boardID, &boardName, &threadID, &title, &threadURL, &targetBook)
	if err != nil {
		return Task{}, err
	}
	var active int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM novel_crawl_import_task WHERE candidate_id=$1 AND status IN ('pending','running')`, in.CandidateID).Scan(&active); err != nil {
		return Task{}, err
	}
	if active > 0 {
		return Task{}, errors.New("candidate already has active import task")
	}
	if in.DisplayTitle == "" {
		in.DisplayTitle = title
	}
	var task Task
	err = scan(tx.QueryRowContext(ctx, `INSERT INTO novel_crawl_import_task(candidate_id,source_id,source_name,board_id,board_name,forum_thread_id,thread_title,display_title,thread_url,target_book_id,import_mode,merge_strategy,status,operator_name) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,'')::bigint,$11,$12,'pending',$13) RETURNING `+returningCols, in.CandidateID, sourceID, sourceName, boardID, boardName, threadID, title, in.DisplayTitle, threadURL, in.TargetBookID, in.ImportMode, in.MergeStrategy, in.OperatorName), &task)
	if err != nil {
		return Task{}, err
	}
	payload, _ := json.Marshal(map[string]string{"importTaskId": task.ID, "candidateId": in.CandidateID})
	key := "import-task:" + task.ID
	var jobID string
	if err = tx.QueryRowContext(ctx, `INSERT INTO platform_jobs(module,job_type,idempotency_key,payload,max_attempts) VALUES('novel','forum_import',$1,$2,3) RETURNING id::text`, key, payload).Scan(&jobID); err != nil {
		return Task{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE novel_crawl_import_task SET platform_job_id=$1 WHERE id=$2`, jobID, task.ID); err != nil {
		return Task{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE novel_crawl_thread_candidate SET import_task_id=$1,status='importing',updated_at=now() WHERE id=$2`, task.ID, in.CandidateID); err != nil {
		return Task{}, err
	}
	if err = tx.Commit(); err != nil {
		return Task{}, err
	}
	task.Status = "pending"
	return task, nil
}
func (r SQLRepository) Retry(ctx context.Context, id int64, in CreateInput) (Task, error) {
	var v Task
	err := r.DB.QueryRowContext(ctx, `UPDATE novel_crawl_import_task SET status='pending',fail_reason='',end_time=NULL,updated_at=now() WHERE id=$1 AND status IN ('failed','cancelled') RETURNING `+returningCols, id).Scan(&v.ID, &v.CandidateID, &v.SourceID, &v.SourceName, &v.BoardID, &v.BoardName, &v.ForumThreadID, &v.ThreadTitle, &v.DisplayTitle, &v.ThreadURL, &v.TargetBookID, &v.ImportMode, &v.MergeStrategy, &v.Status, &v.QualityStatus, &v.TotalChapterCount, &v.ImportedChapterCount, &v.EmptyChapterCount, &v.DuplicateChapterCount, &v.AttemptCount, &v.MaxAttempts, &v.QualitySummary, &v.FailReason, &v.OperatorName, &v.StartTime, &v.EndTime, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return Task{}, err
	}
	return v, nil
}
func (r SQLRepository) Cancel(ctx context.Context, id int64) error {
	res, err := r.DB.ExecContext(ctx, `UPDATE novel_crawl_import_task SET status='cancelled',end_time=now(),updated_at=now() WHERE id=$1 AND status IN ('pending','running')`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("import task cannot be cancelled")
	}
	return nil
}
