package objectstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/jobs"
)

const (
	GCJobModule = "novel"
	GCJobType   = "object_gc"
)

type GCWorkerConfig struct {
	WorkerID     string
	Interval     time.Duration
	Lease        time.Duration
	GracePeriod  time.Duration
	BatchSize    int
	PollInterval time.Duration
}

type GCWorker struct {
	jobs    *jobs.Repository
	objects *Service
	config  GCWorkerConfig
	now     func() time.Time
}

func NewGCWorker(db *sql.DB, objects *Service, config GCWorkerConfig) (*GCWorker, error) {
	if db == nil || objects == nil {
		return nil, errors.New("object GC database and service are required")
	}
	if config.WorkerID == "" || config.Interval <= 0 || config.Lease <= 0 || config.GracePeriod < 0 || config.BatchSize < 1 || config.BatchSize > 1000 || config.PollInterval <= 0 {
		return nil, errors.New("object GC worker configuration is invalid")
	}
	return &GCWorker{jobs: jobs.NewRepository(db), objects: objects, config: config, now: time.Now}, nil
}

func (worker *GCWorker) Run(ctx context.Context) error {
	if err := worker.runCycle(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	ticker := time.NewTicker(worker.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := worker.runCycle(ctx); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
		}
	}
}

func (worker *GCWorker) runCycle(ctx context.Context) error {
	now := worker.now().UTC()
	key := now.Truncate(worker.config.Interval).Format(time.RFC3339)
	payload, _ := json.Marshal(map[string]string{"before": now.Add(-worker.config.GracePeriod).Format(time.RFC3339Nano)})
	if _, _, err := worker.jobs.Enqueue(ctx, jobs.EnqueueOptions{Module: GCJobModule, Type: GCJobType, IdempotencyKey: key, Payload: payload, MaxAttempts: 3}); err != nil {
		return fmt.Errorf("enqueue object GC: %w", err)
	}
	for {
		job, err := worker.jobs.Claim(ctx, jobs.ClaimOptions{WorkerID: worker.config.WorkerID, Module: GCJobModule, Types: []string{GCJobType}, LeaseDuration: worker.config.Lease})
		if errors.Is(err, jobs.ErrNoJob) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("claim object GC: %w", err)
		}
		var input struct {
			Before time.Time `json:"before"`
		}
		if err := json.Unmarshal(job.Payload, &input); err != nil || input.Before.IsZero() {
			_ = worker.jobs.Fail(ctx, job.ID, worker.config.WorkerID, jobs.Failure{Code: "INVALID_PAYLOAD", Message: "object GC payload is invalid", Retryable: false})
			continue
		}
		total := CollectResult{}
		for {
			if err := worker.jobs.Renew(ctx, job.ID, worker.config.WorkerID, worker.config.Lease); err != nil {
				return fmt.Errorf("renew object GC lease: %w", err)
			}
			result, collectErr := worker.objects.Collect(ctx, input.Before, worker.config.BatchSize)
			total.Examined += result.Examined
			total.Deleted += result.Deleted
			total.Failed += result.Failed
			if collectErr != nil || result.Failed > 0 {
				if err := worker.jobs.Fail(ctx, job.ID, worker.config.WorkerID, jobs.Failure{Code: "OBJECT_GC_FAILED", Message: "object garbage collection failed", Retryable: true}); err != nil {
					return err
				}
				break
			}
			if result.Examined < worker.config.BatchSize {
				encoded, _ := json.Marshal(total)
				if err := worker.jobs.Complete(ctx, job.ID, worker.config.WorkerID, encoded); err != nil {
					return fmt.Errorf("complete object GC: %w", err)
				}
				break
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(worker.config.PollInterval):
			}
		}
	}
}
