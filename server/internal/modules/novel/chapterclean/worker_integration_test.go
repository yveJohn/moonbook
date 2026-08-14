//go:build integration

package chapterclean

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/aiconfig"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/chapters"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/jobs"
)

func TestWorkerRecoversExpiredLeaseWithoutDuplicateCleanResult(t *testing.T) {
	db, config := integrationtest.RequireDB(t)
	minioStore := integrationtest.RequireMinIO(t, db, config)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	suffix := time.Now().UnixNano()
	categoryID, authorID, bookID := suffix, suffix+1, suffix+2
	code := fmt.Sprintf("clean-recovery-%d", suffix)
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_categories(id,code,name,kind,source) VALUES($1,$2,$2,'primary','native')`, categoryID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_authors(id,pen_name,normalized_name,status,source) VALUES($1,$2,$2,'active','native')`, authorID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_books(id,primary_category_id,category_code,category_name,book_name,author_id,author_name) VALUES($1,$2,$3,$3,$3,$4,$3)`, bookID, categoryID, code, authorID); err != nil {
		t.Fatal(err)
	}
	objects := objectstore.NewService(db, minioStore.Store)
	chapterNo := 1
	content := "主角在风雪中踏上旅程。"
	chapter, err := chapters.NewService(db, objects).Create(ctx, chapters.Input{BookID: bookID, ChapterNo: &chapterNo, ChapterName: "第一章", ChapterStatus: "enabled", AICleanStatus: "pending", SourceType: "manual", Content: &content})
	if err != nil {
		t.Fatal(err)
	}

	var aiConfigID, modelID int64
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_ai_config(config_name,base_url,stream_mode,failure_threshold,enabled) VALUES($1,'http://fixture','NON_STREAM',2,true) RETURNING id`, code).Scan(&aiConfigID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_ai_config_model(ai_config_id,model_name,sort_order) VALUES($1,'fixture',1) RETURNING id`, aiConfigID).Scan(&modelID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE novel_ai_config SET current_model_id=$2 WHERE id=$1`, aiConfigID, modelID); err != nil {
		t.Fatal(err)
	}
	service := NewService(db, objects)
	if _, err = service.SaveConfig(ctx, ConfigInput{AIConfigID: strconv.FormatInt(aiConfigID, 10), Enabled: true, SystemPrompt: "clean", Temperature: .1, MinCleanedTextPercent: 70, AutoSuccessWordCount: 1, TimeoutSeconds: 30, RetryCount: 1}); err != nil {
		t.Fatal(err)
	}
	task, err := service.Start(ctx, StartInput{BookID: strconv.FormatInt(bookID, 10), OperatorName: "integration"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = db.ExecContext(cleanup, `DELETE FROM platform_job_attempts WHERE job_id IN (SELECT id FROM platform_jobs WHERE module='novel' AND job_type='chapter_clean' AND idempotency_key=$1)`, "chapter-clean:"+task.ID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM platform_jobs WHERE module='novel' AND job_type='chapter_clean' AND idempotency_key=$1`, "chapter-clean:"+task.ID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_chapter_clean_result WHERE task_id=$1`, task.ID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_chapter_clean_task WHERE id=$1`, task.ID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_object_references WHERE book_id=$1`, bookID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_object_events WHERE object_id IN (SELECT id FROM novel_objects WHERE book_id=$1)`, bookID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_objects WHERE book_id=$1`, bookID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_chapters WHERE book_id=$1`, bookID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_books WHERE id=$1`, bookID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_authors WHERE id=$1`, authorID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_categories WHERE id=$1`, categoryID)
	})

	if _, err = db.ExecContext(ctx, `UPDATE platform_jobs SET available_at='1900-01-01 00:00:00+00' WHERE module='novel' AND job_type='chapter_clean' AND idempotency_key=$1`, "chapter-clean:"+task.ID); err != nil {
		t.Fatal(err)
	}
	deadJob, err := service.Jobs.Claim(ctx, jobs.ClaimOptions{WorkerID: "clean-integration-dead", Module: "novel", Types: []string{"chapter_clean"}, LeaseDuration: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE platform_jobs SET lease_expires_at=now()-interval '1 second' WHERE id=$1`, deadJob.ID); err != nil {
		t.Fatal(err)
	}
	if _, recoverErr := service.Jobs.RecoverExpired(ctx); recoverErr != nil {
		t.Fatal(recoverErr)
	}
	if _, err = db.ExecContext(ctx, `UPDATE platform_jobs SET available_at='1900-01-01 00:00:00+00' WHERE id=$1`, deadJob.ID); err != nil {
		t.Fatal(err)
	}

	worker := Worker{DB: db, Jobs: service.Jobs, AI: aiconfig.NewService(db, nil), Objects: objects, WorkerID: "clean-integration-replacement", Lease: time.Minute}
	worked, err := worker.RunOnce(ctx)
	if err != nil || !worked {
		t.Fatalf("worked=%t err=%v", worked, err)
	}

	var taskStatus, chapterStatus, jobStatus string
	var processed, succeeded, attempts, resultRows, activeResults int
	if err = db.QueryRowContext(ctx, `SELECT status,processed_count,success_count FROM novel_chapter_clean_task WHERE id=$1`, task.ID).Scan(&taskStatus, &processed, &succeeded); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT ai_clean_status FROM novel_chapters WHERE id=$1`, chapter.ID).Scan(&chapterStatus); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT status,attempt_count FROM platform_jobs WHERE module='novel' AND job_type='chapter_clean' AND idempotency_key=$1`, "chapter-clean:"+task.ID).Scan(&jobStatus, &attempts); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT count(*),count(*) FILTER (WHERE active) FROM novel_chapter_clean_result WHERE task_id=$1 AND chapter_id=$2`, task.ID, chapter.ID).Scan(&resultRows, &activeResults); err != nil {
		t.Fatal(err)
	}
	if taskStatus != "completed" || chapterStatus != "cleaned" || jobStatus != "succeeded" || processed != 1 || succeeded != 1 || attempts != 2 || resultRows != 1 || activeResults != 1 {
		t.Fatalf("task=%s chapter=%s job=%s processed=%d succeeded=%d attempts=%d results=%d active=%d", taskStatus, chapterStatus, jobStatus, processed, succeeded, attempts, resultRows, activeResults)
	}
}
