package txtimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type SQLRepository struct{ DB *sql.DB }

const columns = `t.id::text,t.target_book_id::text,t.original_filename,t.object_key,t.object_sha256,t.object_byte_size::text,t.status,t.quality_status,t.quality_summary,t.total_chapter_count::text,t.imported_chapter_count::text,t.empty_chapter_count::text,t.duplicate_chapter_count::text,t.fail_reason,t.operator_name,t.attempt_count::text,t.max_attempts::text,COALESCE(t.start_time::text,''),COALESCE(t.end_time::text,''),t.created_at::text,t.updated_at::text`
const returningColumns = `id::text,target_book_id::text,original_filename,object_key,object_sha256,object_byte_size::text,status,quality_status,quality_summary,total_chapter_count::text,imported_chapter_count::text,empty_chapter_count::text,duplicate_chapter_count::text,fail_reason,operator_name,attempt_count::text,max_attempts::text,COALESCE(start_time::text,''),COALESCE(end_time::text,''),created_at::text,updated_at::text`

func scan(row interface{ Scan(...any) error }, v *Task) error {
	return row.Scan(&v.ID, &v.TargetBookID, &v.OriginalFilename, &v.ObjectKey, &v.ObjectSHA256, &v.ObjectByteSize, &v.Status, &v.QualityStatus, &v.QualitySummary, &v.TotalChapterCount, &v.ImportedChapterCount, &v.EmptyChapterCount, &v.DuplicateChapterCount, &v.FailReason, &v.OperatorName, &v.AttemptCount, &v.MaxAttempts, &v.StartTime, &v.EndTime, &v.CreatedAt, &v.UpdatedAt)
}

func (r SQLRepository) List(ctx context.Context, keyword, status string, page, size int) ([]Task, int64, error) {
	where := ` WHERE ($1='' OR t.original_filename ILIKE '%'||$1||'%') AND ($2='' OR t.status=$2)`
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM novel_txt_import_task t`+where, keyword, status).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT `+columns+` FROM novel_txt_import_task t`+where+` ORDER BY t.created_at DESC,t.id DESC LIMIT $3 OFFSET $4`, keyword, status, size, (page-1)*size)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []Task{}
	for rows.Next() {
		var item Task
		if err := scan(rows, &item); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r SQLRepository) Get(ctx context.Context, id int64) (Task, error) {
	var item Task
	err := scan(r.DB.QueryRowContext(ctx, `SELECT `+columns+` FROM novel_txt_import_task t WHERE t.id=$1`, id), &item)
	return item, err
}

func (r SQLRepository) Create(ctx context.Context, in CreateInput) (Task, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback()
	var task Task
	err = scan(tx.QueryRowContext(ctx, `INSERT INTO novel_txt_import_task(target_book_id,original_filename,object_key,object_sha256,object_byte_size,operator_name) VALUES($1,$2,$3,$4,$5,$6) RETURNING `+returningColumns, in.TargetBookID, in.OriginalFilename, in.Object.Key, in.Object.SHA256, in.Object.ByteSize, in.OperatorName), &task)
	if err != nil {
		return Task{}, err
	}
	payload, _ := json.Marshal(map[string]string{"txtImportTaskId": task.ID})
	var jobID string
	if err = tx.QueryRowContext(ctx, `INSERT INTO platform_jobs(module,job_type,idempotency_key,payload,max_attempts) VALUES('novel','txt_import',$1,$2,3) RETURNING id::text`, "txt-import:"+task.ID, payload).Scan(&jobID); err != nil {
		return Task{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE novel_txt_import_task SET platform_job_id=$1 WHERE id=$2`, jobID, task.ID); err != nil {
		return Task{}, err
	}
	if err = tx.Commit(); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (r SQLRepository) Retry(ctx context.Context, id int64) (Task, error) {
	var item Task
	err := scan(r.DB.QueryRowContext(ctx, `UPDATE novel_txt_import_task SET status='pending',quality_status='pending',quality_summary='',fail_reason='',end_time=NULL,updated_at=now() WHERE id=$1 AND status IN ('failed','cancelled') RETURNING `+returningColumns, id), &item)
	if err != nil {
		return Task{}, err
	}
	if _, err = r.DB.ExecContext(ctx, `UPDATE platform_jobs SET status='pending',available_at=now(),lease_owner=NULL,lease_expires_at=NULL,finished_at=NULL,updated_at=now() WHERE module='novel' AND job_type='txt_import' AND payload->>'txtImportTaskId'=$1`, item.ID); err != nil {
		return Task{}, err
	}
	return item, nil
}

func (r SQLRepository) Cancel(ctx context.Context, id int64) error {
	result, err := r.DB.ExecContext(ctx, `UPDATE novel_txt_import_task SET status='cancelled',end_time=now(),updated_at=now() WHERE id=$1 AND status IN ('pending','running')`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return errors.New("TXT import task cannot be cancelled")
	}
	return nil
}
