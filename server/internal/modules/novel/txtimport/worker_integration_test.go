//go:build integration

package txtimport

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/chapters"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/importtask"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/jobs"
)

func TestWorkerRecoversExpiredLeaseWithoutDuplicateChapters(t *testing.T) {
	db, config := integrationtest.RequireDB(t)
	minioStore := integrationtest.RequireMinIO(t, db, config)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	suffix := time.Now().UnixNano()
	categoryID, authorID, bookID := suffix, suffix+1, suffix+2
	code := fmt.Sprintf("txt-recovery-%d", suffix)
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_categories(id,code,name,kind,source) VALUES($1,$2,$2,'primary','native')`, categoryID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_authors(id,pen_name,normalized_name,status,source) VALUES($1,$2,$2,'active','native')`, authorID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_books(id,primary_category_id,category_code,category_name,book_name,author_id,author_name) VALUES($1,$2,$3,$3,$3,$4,$3)`, bookID, categoryID, code, authorID); err != nil {
		t.Fatal(err)
	}

	fileStore := MinIOFileStore{Blobs: minioStore.Store}
	data := []byte("第1章 初见\n月色落在窗台。\n第2章 远行\n列车驶向远方。")
	meta, err := fileStore.Put(ctx, "imports/txt/"+code+".txt", data, "text/plain; charset=utf-8")
	if err != nil {
		t.Fatal(err)
	}
	task, err := (SQLRepository{DB: db}).Create(ctx, CreateInput{TargetBookID: strconv.FormatInt(bookID, 10), OriginalFilename: "recovery.txt", Object: meta, OperatorName: "integration"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = db.ExecContext(cleanup, `DELETE FROM platform_job_attempts WHERE job_id IN (SELECT id FROM platform_jobs WHERE module='novel' AND job_type=$1 AND idempotency_key=$2)`, JobType, "txt-import:"+task.ID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM platform_jobs WHERE module='novel' AND job_type=$1 AND idempotency_key=$2`, JobType, "txt-import:"+task.ID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_txt_import_task WHERE id=$1`, task.ID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_object_references WHERE book_id=$1`, bookID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_object_events WHERE object_id IN (SELECT id FROM novel_objects WHERE book_id=$1)`, bookID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_objects WHERE book_id=$1`, bookID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_chapters WHERE book_id=$1`, bookID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_books WHERE id=$1`, bookID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_authors WHERE id=$1`, authorID)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_categories WHERE id=$1`, categoryID)
	})

	repository := jobs.NewRepository(db)
	if _, err = db.ExecContext(ctx, `UPDATE platform_jobs SET available_at='1900-01-01 00:00:00+00' WHERE module='novel' AND job_type=$1 AND idempotency_key=$2`, JobType, "txt-import:"+task.ID); err != nil {
		t.Fatal(err)
	}
	deadJob, err := repository.Claim(ctx, jobs.ClaimOptions{WorkerID: "txt-integration-dead", Module: "novel", Types: []string{JobType}, LeaseDuration: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE platform_jobs SET lease_expires_at=now()-interval '1 second' WHERE id=$1`, deadJob.ID); err != nil {
		t.Fatal(err)
	}
	if _, recoverErr := repository.RecoverExpired(ctx); recoverErr != nil {
		t.Fatal(recoverErr)
	}
	if _, err = db.ExecContext(ctx, `UPDATE platform_jobs SET available_at='1900-01-01 00:00:00+00' WHERE id=$1`, deadJob.ID); err != nil {
		t.Fatal(err)
	}

	objects := objectstore.NewService(db, minioStore.Store)
	worker := Worker{
		DB:       db,
		Jobs:     repository,
		Store:    fileStore,
		Writer:   importtask.ChaptersWriter{Service: chapters.NewService(db, objects)},
		WorkerID: "txt-integration-replacement",
		Lease:    time.Minute,
	}
	worked, err := worker.RunOnce(ctx)
	if err != nil || !worked {
		t.Fatalf("worked=%t err=%v", worked, err)
	}

	var taskStatus, jobStatus string
	var attempts, chapterCount, activeObjects int
	if err = db.QueryRowContext(ctx, `SELECT status FROM novel_txt_import_task WHERE id=$1`, task.ID).Scan(&taskStatus); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT status,attempt_count FROM platform_jobs WHERE module='novel' AND job_type=$1 AND idempotency_key=$2`, JobType, "txt-import:"+task.ID).Scan(&jobStatus, &attempts); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM novel_chapters WHERE book_id=$1 AND deleted_at IS NULL`, bookID).Scan(&chapterCount); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM novel_object_references r JOIN novel_objects o ON o.id=r.object_id WHERE r.book_id=$1 AND o.state='active'`, bookID).Scan(&activeObjects); err != nil {
		t.Fatal(err)
	}
	if taskStatus != "succeeded" || jobStatus != "succeeded" || attempts != 2 || chapterCount != 2 || activeObjects != 2 {
		t.Fatalf("task=%s job=%s attempts=%d chapters=%d activeObjects=%d", taskStatus, jobStatus, attempts, chapterCount, activeObjects)
	}
}
