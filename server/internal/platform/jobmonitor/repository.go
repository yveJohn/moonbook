package jobmonitor

import (
	"context"
	"database/sql"
)

type SQLRepository struct{ DB *sql.DB }

const jobColumns = `j.id::text,j.module,j.job_type,j.status,j.attempt_count,j.max_attempts,
	j.available_at::text,COALESCE(j.lease_owner,''),COALESCE(j.lease_expires_at::text,''),
	COALESCE(j.last_error_code,''),COALESCE(j.last_error_message,''),j.created_at::text,j.updated_at::text,
	COALESCE(j.finished_at::text,'')`

func scanJob(row interface{ Scan(...any) error }, job *Job) error {
	return row.Scan(&job.ID, &job.Module, &job.JobType, &job.Status, &job.AttemptCount, &job.MaxAttempts,
		&job.AvailableAt, &job.LeaseOwner, &job.LeaseExpiresAt, &job.LastErrorCode, &job.LastErrorMessage,
		&job.CreatedAt, &job.UpdatedAt, &job.FinishedAt)
}

func (r SQLRepository) List(ctx context.Context, f Filters, page, size int) ([]Job, int64, error) {
	where := ` WHERE ($1='' OR j.module=$1) AND ($2='' OR j.job_type=$2) AND ($3='' OR j.status=$3)
		AND ($4='' OR j.lease_owner ILIKE '%'||$4||'%')
		AND ($5='' OR j.created_at >= NULLIF($5,'')::timestamptz)
		AND ($6='' OR j.created_at < NULLIF($6,'')::timestamptz)`
	args := []any{f.Module, f.JobType, f.Status, f.LeaseOwner, f.From, f.To}
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM platform_jobs j`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT `+jobColumns+` FROM platform_jobs j`+where+` ORDER BY j.updated_at DESC,j.id DESC LIMIT $7 OFFSET $8`, append(args, size, (page-1)*size)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]Job, 0)
	for rows.Next() {
		var job Job
		if err := scanJob(rows, &job); err != nil {
			return nil, 0, err
		}
		out = append(out, job)
	}
	return out, total, rows.Err()
}

func (r SQLRepository) Get(ctx context.Context, id string) (Detail, error) {
	var detail Detail
	if err := scanJob(r.DB.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM platform_jobs j WHERE j.id=$1`, id), &detail.Job); err != nil {
		return Detail{}, err
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT id::text,job_id::text,attempt_number,worker_id,started_at::text,
		COALESCE(finished_at::text,''),COALESCE(outcome,''),COALESCE(error_code,''),COALESCE(error_message,'')
		FROM platform_job_attempts WHERE job_id=$1 ORDER BY attempt_number DESC,id DESC`, id)
	if err != nil {
		return Detail{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var attempt Attempt
		if err := rows.Scan(&attempt.ID, &attempt.JobID, &attempt.AttemptNumber, &attempt.WorkerID, &attempt.StartedAt,
			&attempt.FinishedAt, &attempt.Outcome, &attempt.ErrorCode, &attempt.ErrorMessage); err != nil {
			return Detail{}, err
		}
		detail.Attempts = append(detail.Attempts, attempt)
	}
	return detail, rows.Err()
}
