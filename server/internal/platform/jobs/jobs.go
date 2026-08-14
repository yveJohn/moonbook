package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrNoJob     = errors.New("no job is available")
	ErrLeaseLost = errors.New("job lease is no longer owned by this worker")
)

type Job struct {
	ID             int64
	Module         string
	Type           string
	IdempotencyKey string
	Status         string
	Payload        json.RawMessage
	AttemptCount   int
	MaxAttempts    int
	AvailableAt    time.Time
	LeaseOwner     sql.NullString
	LeaseExpiresAt sql.NullTime
}

type EnqueueOptions struct {
	Module         string
	Type           string
	IdempotencyKey string
	Payload        json.RawMessage
	MaxAttempts    int
	AvailableAt    time.Time
}

type Failure struct {
	Code      string
	Message   string
	Retryable bool
}

type ClaimOptions struct {
	WorkerID      string
	Module        string
	Types         []string
	LeaseDuration time.Duration
}

type Repository struct {
	db          *sql.DB
	baseBackoff time.Duration
	maxBackoff  time.Duration
}

// Heartbeat keeps a claimed job lease alive and cancels Context when the
// repository can no longer prove ownership of the lease.
type Heartbeat struct {
	Context context.Context
	cancel  context.CancelFunc
	done    chan error
	once    sync.Once
	err     error
}

func (repository *Repository) KeepAlive(parent context.Context, jobID int64, workerID string, leaseDuration time.Duration) (*Heartbeat, error) {
	workerID = strings.TrimSpace(workerID)
	if jobID <= 0 || workerID == "" || leaseDuration < time.Millisecond {
		return nil, errors.New("job ID, worker ID, and lease duration of at least one millisecond are required")
	}
	ctx, cancel := context.WithCancel(parent)
	heartbeat := &Heartbeat{Context: ctx, cancel: cancel, done: make(chan error, 1)}
	interval := leaseDuration / 3
	if interval <= 0 {
		interval = time.Nanosecond
	}
	if interval < 100*time.Millisecond && leaseDuration >= 300*time.Millisecond {
		interval = 100 * time.Millisecond
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				heartbeat.done <- nil
				return
			case <-ticker.C:
				if err := repository.Renew(ctx, jobID, workerID, leaseDuration); err != nil {
					if ctx.Err() != nil {
						heartbeat.done <- nil
					} else {
						heartbeat.done <- err
						heartbeat.cancel()
					}
					return
				}
			}
		}
	}()
	return heartbeat, nil
}

func (heartbeat *Heartbeat) Stop() error {
	if heartbeat == nil {
		return nil
	}
	heartbeat.once.Do(func() {
		heartbeat.cancel()
		heartbeat.err = <-heartbeat.done
	})
	return heartbeat.err
}

// Finalize keeps renewal active while the caller persists the terminal job
// state. A successful finalizer proves ownership, so a concurrent renewal
// observing that new terminal state is not treated as lease loss.
func (heartbeat *Heartbeat) Finalize(finalizer func() error) error {
	if heartbeat == nil || finalizer == nil {
		return errors.New("job heartbeat and finalizer are required")
	}
	finalizeErr := finalizer()
	heartbeatErr := heartbeat.Stop()
	if finalizeErr != nil {
		if heartbeatErr != nil {
			return heartbeatErr
		}
		return finalizeErr
	}
	return nil
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, baseBackoff: time.Second, maxBackoff: 15 * time.Minute}
}

func (repository *Repository) Enqueue(ctx context.Context, options EnqueueOptions) (Job, bool, error) {
	options.Module = strings.TrimSpace(options.Module)
	options.Type = strings.TrimSpace(options.Type)
	options.IdempotencyKey = strings.TrimSpace(options.IdempotencyKey)
	if options.Module == "" || options.Type == "" || options.IdempotencyKey == "" {
		return Job{}, false, errors.New("module, type and idempotency key are required")
	}
	if len(options.Module) > 64 || len(options.Type) > 64 || len(options.IdempotencyKey) > 191 {
		return Job{}, false, errors.New("job identity exceeds database limits")
	}
	if len(options.Payload) == 0 {
		options.Payload = json.RawMessage(`{}`)
	}
	if !json.Valid(options.Payload) {
		return Job{}, false, errors.New("payload must be valid JSON")
	}
	if options.MaxAttempts <= 0 {
		options.MaxAttempts = 3
	}
	if options.AvailableAt.IsZero() {
		options.AvailableAt = time.Now().UTC()
	}

	tx, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Job{}, false, fmt.Errorf("begin enqueue: %w", err)
	}
	defer tx.Rollback()
	var job Job
	created := true
	err = tx.QueryRowContext(ctx, `
		INSERT INTO platform_jobs
			(module, job_type, idempotency_key, payload, max_attempts, available_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (module, job_type, idempotency_key) DO NOTHING
		RETURNING id, module, job_type, idempotency_key, status, payload, attempt_count,
			max_attempts, available_at, lease_owner, lease_expires_at`,
		options.Module, options.Type, options.IdempotencyKey, options.Payload, options.MaxAttempts, options.AvailableAt).
		Scan(&job.ID, &job.Module, &job.Type, &job.IdempotencyKey, &job.Status, &job.Payload,
			&job.AttemptCount, &job.MaxAttempts, &job.AvailableAt, &job.LeaseOwner, &job.LeaseExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		created = false
		err = tx.QueryRowContext(ctx, `
			SELECT id, module, job_type, idempotency_key, status, payload, attempt_count,
				max_attempts, available_at, lease_owner, lease_expires_at
			FROM platform_jobs
			WHERE module = $1 AND job_type = $2 AND idempotency_key = $3`,
			options.Module, options.Type, options.IdempotencyKey).
			Scan(&job.ID, &job.Module, &job.Type, &job.IdempotencyKey, &job.Status, &job.Payload,
				&job.AttemptCount, &job.MaxAttempts, &job.AvailableAt, &job.LeaseOwner, &job.LeaseExpiresAt)
	}
	if err != nil {
		return Job{}, false, fmt.Errorf("enqueue job: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Job{}, false, fmt.Errorf("commit enqueue: %w", err)
	}
	return job, created, nil
}

func (repository *Repository) Claim(ctx context.Context, options ClaimOptions) (Job, error) {
	options.WorkerID = strings.TrimSpace(options.WorkerID)
	options.Module = strings.TrimSpace(options.Module)
	if options.WorkerID == "" || len(options.WorkerID) > 128 {
		return Job{}, errors.New("worker ID must contain 1 to 128 characters")
	}
	if options.Module == "" || len(options.Module) > 64 {
		return Job{}, errors.New("module must contain 1 to 64 characters")
	}
	if len(options.Types) == 0 {
		return Job{}, errors.New("at least one job type is required")
	}
	for _, jobType := range options.Types {
		if strings.TrimSpace(jobType) == "" || len(jobType) > 64 {
			return Job{}, errors.New("job types must contain 1 to 64 characters")
		}
	}
	if options.LeaseDuration < time.Millisecond {
		return Job{}, errors.New("lease duration must be at least one millisecond")
	}
	tx, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return Job{}, fmt.Errorf("begin claim: %w", err)
	}
	defer tx.Rollback()

	var job Job
	err = tx.QueryRowContext(ctx, `
		SELECT id, module, job_type, idempotency_key, status, payload, attempt_count,
			max_attempts, available_at, lease_owner, lease_expires_at
		FROM platform_jobs
		WHERE module = $1 AND job_type = ANY($2)
		  AND attempt_count < max_attempts
		  AND available_at <= now()
		  AND (status = 'pending' OR (status = 'running' AND lease_expires_at < now()))
		ORDER BY available_at, id
		FOR UPDATE SKIP LOCKED
		LIMIT 1`, options.Module, options.Types).Scan(&job.ID, &job.Module, &job.Type, &job.IdempotencyKey, &job.Status, &job.Payload,
		&job.AttemptCount, &job.MaxAttempts, &job.AvailableAt, &job.LeaseOwner, &job.LeaseExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNoJob
	}
	if err != nil {
		return Job{}, fmt.Errorf("select claimable job: %w", err)
	}

	job.AttemptCount++
	if err := tx.QueryRowContext(ctx, `
		UPDATE platform_jobs
		SET status = 'running', attempt_count = $2, lease_owner = $3,
			lease_expires_at = now() + ($4 * interval '1 millisecond'), updated_at = now(), last_error_code = NULL,
			last_error_message = NULL
		WHERE id = $1
		RETURNING lease_expires_at`, job.ID, job.AttemptCount, options.WorkerID, options.LeaseDuration.Milliseconds()).Scan(&job.LeaseExpiresAt); err != nil {
		return Job{}, fmt.Errorf("claim job: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO platform_job_attempts (job_id, attempt_number, worker_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (job_id, attempt_number) DO UPDATE
		SET worker_id = EXCLUDED.worker_id, started_at = now(), finished_at = NULL,
			outcome = NULL, error_code = NULL, error_message = NULL`, job.ID, job.AttemptCount, options.WorkerID); err != nil {
		return Job{}, fmt.Errorf("record job attempt: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Job{}, fmt.Errorf("commit claim: %w", err)
	}
	job.Status = "running"
	job.LeaseOwner = sql.NullString{String: options.WorkerID, Valid: true}
	return job, nil
}

func (repository *Repository) Renew(ctx context.Context, jobID int64, workerID string, leaseDuration time.Duration) error {
	if leaseDuration < time.Millisecond {
		return errors.New("lease duration must be at least one millisecond")
	}
	result, err := repository.db.ExecContext(ctx, `
		UPDATE platform_jobs
		SET lease_expires_at = now() + ($3 * interval '1 millisecond'), updated_at = now()
		WHERE id = $1 AND status = 'running' AND lease_owner = $2 AND lease_expires_at > now()`,
		jobID, workerID, leaseDuration.Milliseconds())
	return leaseResult("renew", result, err)
}

func (repository *Repository) RecoverExpired(ctx context.Context) (int64, error) {
	tx, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return 0, fmt.Errorf("begin expired job recovery: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `
		SELECT id, attempt_count, max_attempts
		FROM platform_jobs
		WHERE status = 'running' AND lease_expires_at < now()
		ORDER BY id
		FOR UPDATE`)
	if err != nil {
		return 0, fmt.Errorf("select expired jobs: %w", err)
	}
	type expiredJob struct {
		id, attemptCount, maxAttempts int64
	}
	var expired []expiredJob
	for rows.Next() {
		var job expiredJob
		if err := rows.Scan(&job.id, &job.attemptCount, &job.maxAttempts); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan expired job: %w", err)
		}
		expired = append(expired, job)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("iterate expired jobs: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close expired jobs: %w", err)
	}
	for _, job := range expired {
		status, outcome := "pending", "retry"
		if job.attemptCount >= job.maxAttempts {
			status, outcome = "failed", "failed"
		}
		if _, err := tx.ExecContext(ctx, `
		UPDATE platform_jobs
		SET status = $2::varchar,
			available_at = now(), lease_owner = NULL, lease_expires_at = NULL,
			last_error_code = 'LEASE_EXPIRED', last_error_message = 'worker lease expired',
			finished_at = CASE WHEN $2::varchar = 'failed' THEN now() ELSE NULL END,
			updated_at = now()
		WHERE id = $1`, job.id, status); err != nil {
			return 0, fmt.Errorf("recover expired job %d: %w", job.id, err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE platform_job_attempts
			SET finished_at = now(), outcome = $3, error_code = 'LEASE_EXPIRED',
				error_message = 'worker lease expired'
			WHERE job_id = $1 AND attempt_number = $2`, job.id, job.attemptCount, outcome); err != nil {
			return 0, fmt.Errorf("close expired attempt for job %d: %w", job.id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit expired job recovery: %w", err)
	}
	return int64(len(expired)), nil
}

func (repository *Repository) Complete(ctx context.Context, jobID int64, workerID string, result json.RawMessage) error {
	if len(result) == 0 {
		result = json.RawMessage(`{}`)
	}
	if !json.Valid(result) {
		return errors.New("result must be valid JSON")
	}
	return repository.finish(ctx, jobID, workerID, "succeeded", result, Failure{})
}

func (repository *Repository) Fail(ctx context.Context, jobID int64, workerID string, failure Failure) error {
	failure.Code = strings.TrimSpace(failure.Code)
	failure.Message = strings.TrimSpace(failure.Message)
	if failure.Code == "" || len(failure.Code) > 64 {
		return errors.New("failure code must contain 1 to 64 characters")
	}
	return repository.finish(ctx, jobID, workerID, "", nil, failure)
}

func (repository *Repository) finish(ctx context.Context, jobID int64, workerID, successStatus string, result json.RawMessage, failure Failure) error {
	tx, err := repository.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin job completion: %w", err)
	}
	defer tx.Rollback()
	var attemptCount, maxAttempts int
	err = tx.QueryRowContext(ctx, `
		SELECT attempt_count, max_attempts FROM platform_jobs
		WHERE id = $1 AND status = 'running' AND lease_owner = $2 AND lease_expires_at > now()
		FOR UPDATE`, jobID, workerID).Scan(&attemptCount, &maxAttempts)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrLeaseLost
	}
	if err != nil {
		return fmt.Errorf("lock job completion: %w", err)
	}

	status, outcome := successStatus, "succeeded"
	availableAt := time.Now().UTC()
	if successStatus == "" {
		status, outcome = "failed", "failed"
		if failure.Retryable && attemptCount < maxAttempts {
			status, outcome = "pending", "retry"
			availableAt = availableAt.Add(repository.Backoff(attemptCount))
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE platform_jobs
		SET status = $3::varchar, result = $4, available_at = $5, lease_owner = NULL,
			lease_expires_at = NULL, last_error_code = NULLIF($6, ''),
			last_error_message = NULLIF($7, ''), updated_at = now(),
			finished_at = CASE WHEN $3::varchar IN ('succeeded', 'failed') THEN now() ELSE NULL END
		WHERE id = $1 AND lease_owner = $2`, jobID, workerID, status, result, availableAt, failure.Code, failure.Message); err != nil {
		return fmt.Errorf("update job completion: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE platform_job_attempts
		SET finished_at = now(), outcome = $4, error_code = NULLIF($5, ''), error_message = NULLIF($6, '')
		WHERE job_id = $1 AND attempt_number = $2 AND worker_id = $3`,
		jobID, attemptCount, workerID, outcome, failure.Code, failure.Message); err != nil {
		return fmt.Errorf("finish job attempt: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit job completion: %w", err)
	}
	return nil
}

func (repository *Repository) Backoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := repository.baseBackoff
	for i := 1; i < attempt && delay < repository.maxBackoff; i++ {
		delay *= 2
		if delay > repository.maxBackoff {
			return repository.maxBackoff
		}
	}
	return delay
}

func leaseResult(action string, result sql.Result, err error) error {
	if err != nil {
		return fmt.Errorf("%s job lease: %w", action, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read %s result: %w", action, err)
	}
	if rows != 1 {
		return ErrLeaseLost
	}
	return nil
}
