//go:build integration

package jobmonitor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestSQLRepositoryListsAndLoadsJobAttempts(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	suffix := time.Now().UnixNano()
	module := fmt.Sprintf("jobmonitor-it-%d", suffix)
	ids := []int64{9007199254740993, 9007199254740994}
	for index, id := range ids {
		_, err := db.ExecContext(ctx, `INSERT INTO platform_jobs
            (id,module,job_type,idempotency_key,status,attempt_count,max_attempts,available_at,lease_owner,lease_expires_at,last_error_code,last_error_message,created_at,updated_at,finished_at)
            VALUES($1,$2,$3,$4,$5,$6,3,$7,$8,$9,$10,$11,$12,$13,$14)`,
			id, module, "fixture", fmt.Sprintf("jobmonitor:%d", id), []string{"failed", "running"}[index], index+1,
			time.Date(2026, 8, 21, 1, index, 0, 0, time.UTC), "worker-a", time.Date(2026, 8, 21, 2, 0, 0, 0, time.UTC),
			"FIXTURE", "request failed api_key=must-not-render", time.Date(2026, 8, 21, 0, index, 0, 0, time.UTC),
			time.Date(2026, 8, 21, 3, index, 0, 0, time.UTC), nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM platform_job_attempts WHERE job_id = ANY($1)`, ids)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM platform_jobs WHERE id = ANY($1)`, ids)
	})
	if _, err := db.ExecContext(ctx, `INSERT INTO platform_job_attempts
        (job_id,attempt_number,worker_id,started_at,finished_at,outcome,error_code,error_message)
        VALUES($1,1,'worker-a',$2,$3,'failed','FIXTURE','Cookie=session-secret')`,
		ids[0], time.Date(2026, 8, 21, 1, 0, 0, 0, time.UTC), time.Date(2026, 8, 21, 1, 1, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}

	repo := SQLRepository{DB: db}
	rows, total, err := repo.List(ctx, Filters{Module: module, Status: "running", LeaseOwner: "worker", From: "2026-08-21T00:00:00Z", To: "2026-08-22T00:00:00Z"}, 1, 20)
	if err != nil || total != 1 || len(rows) != 1 || rows[0].ID != "9007199254740994" {
		t.Fatalf("rows=%+v total=%d err=%v", rows, total, err)
	}

	detail, err := repo.Get(ctx, "9007199254740993")
	if err != nil || detail.ID != "9007199254740993" || len(detail.Attempts) != 1 || detail.Attempts[0].JobID != detail.ID {
		t.Fatalf("detail=%+v err=%v", detail, err)
	}
}
