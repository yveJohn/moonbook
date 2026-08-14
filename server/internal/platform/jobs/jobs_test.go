package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestBackoffIsExponentialAndBounded(t *testing.T) {
	repository := NewRepository(nil)
	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 0, want: time.Second},
		{attempt: 1, want: time.Second},
		{attempt: 2, want: 2 * time.Second},
		{attempt: 5, want: 16 * time.Second},
		{attempt: 99, want: 15 * time.Minute},
	}
	for _, tt := range tests {
		if got := repository.Backoff(tt.attempt); got != tt.want {
			t.Fatalf("Backoff(%d) = %s, want %s", tt.attempt, got, tt.want)
		}
	}
}

func TestConcurrentClaimAndExpiredRecoveryWithPostgres(t *testing.T) {
	dsn := os.Getenv("MOONBOOK_JOBS_TEST_DSN")
	if dsn == "" {
		t.Skip("MOONBOOK_JOBS_TEST_DSN is not configured")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	suffix := time.Now().UTC().Format("150405.000000000")
	module := "concurrency-" + suffix
	jobType := "claim"
	for i := 0; i < 2; i++ {
		if _, created, err := repository.Enqueue(ctx, EnqueueOptions{Module: module, Type: jobType, IdempotencyKey: string(rune('a' + i)), MaxAttempts: 1, AvailableAt: time.Now().Add(-time.Second)}); err != nil || !created {
			t.Fatalf("enqueue %d created=%t err=%v", i, created, err)
		}
	}

	claimed := make(chan Job, 2)
	errs := make(chan error, 2)
	var workers sync.WaitGroup
	for _, workerID := range []string{"concurrent-a", "concurrent-b"} {
		workers.Add(1)
		go func(workerID string) {
			defer workers.Done()
			job, err := repository.Claim(ctx, ClaimOptions{WorkerID: workerID, Module: module, Types: []string{jobType}, LeaseDuration: time.Minute})
			claimed <- job
			errs <- err
		}(workerID)
	}
	workers.Wait()
	close(claimed)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	ids := map[int64]bool{}
	for job := range claimed {
		ids[job.ID] = true
	}
	if len(ids) != 2 {
		t.Fatalf("concurrent workers claimed %d unique jobs, want 2", len(ids))
	}
	retryJob, created, err := repository.Enqueue(ctx, EnqueueOptions{Module: module, Type: jobType, IdempotencyKey: "retryable", MaxAttempts: 2, AvailableAt: time.Now().Add(-time.Second)})
	if err != nil || !created {
		t.Fatalf("enqueue retryable created=%t err=%v", created, err)
	}
	retryClaim, err := repository.Claim(ctx, ClaimOptions{WorkerID: "concurrent-c", Module: module, Types: []string{jobType}, LeaseDuration: time.Minute})
	if err != nil || retryClaim.ID != retryJob.ID {
		t.Fatalf("retry claim=%+v err=%v", retryClaim, err)
	}
	ids[retryClaim.ID] = true

	for jobID := range ids {
		if _, err := db.ExecContext(ctx, "UPDATE platform_jobs SET lease_expires_at=now()-interval '1 minute' WHERE id=$1", jobID); err != nil {
			t.Fatal(err)
		}
	}
	recovered, err := repository.RecoverExpired(ctx)
	if err != nil || recovered < 3 {
		t.Fatalf("RecoverExpired count=%d err=%v", recovered, err)
	}
	rows, err := db.QueryContext(ctx, `
		SELECT j.status, a.outcome, count(*)
		FROM platform_jobs j JOIN platform_job_attempts a ON a.job_id=j.id
		WHERE j.module=$1 GROUP BY j.status,a.outcome`, module)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	groups := map[string]int{}
	for rows.Next() {
		var status, outcome string
		var count int
		if err := rows.Scan(&status, &outcome, &count); err != nil {
			t.Fatal(err)
		}
		groups[status+"/"+outcome] = count
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if groups["failed/failed"] != 2 || groups["pending/retry"] != 1 || len(groups) != 2 {
		t.Fatalf("recovery result groups=%v", groups)
	}
}

func TestEnqueueValidatesBeforeDatabaseAccess(t *testing.T) {
	repository := NewRepository(&sql.DB{})
	tests := []EnqueueOptions{
		{},
		{Module: "novel", Type: "import", IdempotencyKey: "key", Payload: json.RawMessage(`{broken`)},
	}
	for _, options := range tests {
		if _, _, err := repository.Enqueue(context.Background(), options); err == nil {
			t.Fatal("Enqueue returned nil error")
		}
	}
}

func TestLeaseDurationsRejectSubMillisecondValues(t *testing.T) {
	repository := NewRepository(&sql.DB{})
	if _, err := repository.Claim(context.Background(), ClaimOptions{WorkerID: "worker", Module: "module", Types: []string{"type"}, LeaseDuration: time.Nanosecond}); err == nil {
		t.Fatal("Claim accepted a sub-millisecond lease")
	}
	if err := repository.Renew(context.Background(), 1, "worker", time.Nanosecond); err == nil {
		t.Fatal("Renew accepted a sub-millisecond lease")
	}
	if _, err := repository.KeepAlive(context.Background(), 1, "worker", time.Nanosecond); err == nil {
		t.Fatal("KeepAlive accepted a sub-millisecond lease")
	}
}

func TestRepositoryLifecycleWithPostgres(t *testing.T) {
	dsn := os.Getenv("MOONBOOK_JOBS_TEST_DSN")
	if dsn == "" {
		t.Skip("MOONBOOK_JOBS_TEST_DSN is not configured")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	key := "integration-" + time.Now().UTC().Format("20060102T150405.000000000")
	module := "lifecycle-" + key
	jobType := "run"

	job, created, err := repository.Enqueue(ctx, EnqueueOptions{Module: module, Type: jobType, IdempotencyKey: key, Payload: json.RawMessage(`{"bookId":"9223372036854775807"}`), MaxAttempts: 2, AvailableAt: time.Now().Add(-time.Second)})
	if err != nil || !created {
		t.Fatalf("Enqueue created=%t err=%v", created, err)
	}
	duplicate, created, err := repository.Enqueue(ctx, EnqueueOptions{Module: module, Type: jobType, IdempotencyKey: key, Payload: json.RawMessage(`{"ignored":true}`), MaxAttempts: 9})
	if err != nil || created || duplicate.ID != job.ID || duplicate.MaxAttempts != 2 {
		t.Fatalf("duplicate=%+v created=%t err=%v", duplicate, created, err)
	}

	claimOptions := ClaimOptions{WorkerID: "worker-a", Module: module, Types: []string{jobType}, LeaseDuration: time.Minute}
	claimed, err := repository.Claim(ctx, claimOptions)
	if err != nil || claimed.ID != job.ID || claimed.AttemptCount != 1 {
		t.Fatalf("first claim=%+v err=%v", claimed, err)
	}
	if err := repository.Renew(ctx, job.ID, "worker-b", time.Minute); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("foreign renew error=%v", err)
	}
	if err := repository.Fail(ctx, job.ID, "worker-a", Failure{Code: "TEMPORARY", Message: "retry later", Retryable: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE platform_jobs SET available_at = now() WHERE id = $1", job.ID); err != nil {
		t.Fatal(err)
	}
	claimOptions.WorkerID = "worker-b"
	claimed, err = repository.Claim(ctx, claimOptions)
	if err != nil || claimed.ID != job.ID || claimed.AttemptCount != 2 {
		t.Fatalf("second claim=%+v err=%v", claimed, err)
	}
	if err := repository.Complete(ctx, job.ID, "worker-a", json.RawMessage(`{"wrong":true}`)); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("stale complete error=%v", err)
	}
	if err := repository.Complete(ctx, job.ID, "worker-b", json.RawMessage(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	var status string
	var attempts int
	if err := db.QueryRowContext(ctx, "SELECT status, attempt_count FROM platform_jobs WHERE id=$1", job.ID).Scan(&status, &attempts); err != nil {
		t.Fatal(err)
	}
	if status != "succeeded" || attempts != 2 {
		t.Fatalf("status=%s attempts=%d", status, attempts)
	}
}

func TestHeartbeatRenewsAndCancelsWhenLeaseIsLostWithPostgres(t *testing.T) {
	dsn := os.Getenv("MOONBOOK_JOBS_TEST_DSN")
	if dsn == "" {
		t.Skip("MOONBOOK_JOBS_TEST_DSN is not configured")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := NewRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	module := "heartbeat-" + time.Now().UTC().Format("150405.000000000")

	kept, _, err := repository.Enqueue(ctx, EnqueueOptions{Module: module, Type: "run", IdempotencyKey: "kept", MaxAttempts: 1, AvailableAt: time.Now().Add(-time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	kept, err = repository.Claim(ctx, ClaimOptions{WorkerID: "heartbeat-a", Module: module, Types: []string{"run"}, LeaseDuration: 300 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	heartbeat, err := repository.KeepAlive(ctx, kept.ID, "heartbeat-a", 300*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(650 * time.Millisecond)
	if err = heartbeat.Stop(); err != nil {
		t.Fatalf("stop renewed heartbeat: %v", err)
	}
	if err = repository.Complete(ctx, kept.ID, "heartbeat-a", json.RawMessage(`{"renewed":true}`)); err != nil {
		t.Fatalf("complete renewed job: %v", err)
	}

	lost, _, err := repository.Enqueue(ctx, EnqueueOptions{Module: module, Type: "run", IdempotencyKey: "lost", MaxAttempts: 2, AvailableAt: time.Now().Add(-time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	lost, err = repository.Claim(ctx, ClaimOptions{WorkerID: "heartbeat-b", Module: module, Types: []string{"run"}, LeaseDuration: 300 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	heartbeat, err = repository.KeepAlive(ctx, lost.ID, "heartbeat-b", 300*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE platform_jobs SET lease_expires_at=now()-interval '1 second' WHERE id=$1`, lost.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-heartbeat.Context.Done():
	case <-ctx.Done():
		t.Fatal("heartbeat did not cancel after lease loss")
	}
	if err = heartbeat.Stop(); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("lost heartbeat error=%v", err)
	}
}
