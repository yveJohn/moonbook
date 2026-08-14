package chaptersummary

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/aiconfig"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type fixtureSummarizer struct{ content string }

func (summarizer *fixtureSummarizer) Summarize(_ context.Context, _ aiconfig.RuntimeConfig, _ string, content string, _ float64, _ int, _ time.Duration) (string, string, error) {
	summarizer.content = content
	return "主角在密道中找到线索", `{"fixture":true}`, nil
}

func TestConfigLifecyclePostgreSQL(t *testing.T) {
	dsn := os.Getenv("MOONBOOK_CHAPTER_SUMMARY_TEST_DSN")
	if dsn == "" {
		t.Skip("MOONBOOK_CHAPTER_SUMMARY_TEST_DSN is not set")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	var configID, modelID int64
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_ai_config(config_name,base_url,stream_mode,failure_threshold,enabled) VALUES('summary-test','http://127.0.0.1:1','NON_STREAM',2,true) RETURNING id`).Scan(&configID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_ai_config_model(ai_config_id,model_name,sort_order) VALUES($1,'test-model',1) RETURNING id`, configID).Scan(&modelID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE novel_ai_config SET current_model_id=$2 WHERE id=$1`, configID, modelID); err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	input := ConfigInput{AIConfigID: fmt.Sprint(configID), Enabled: false, SystemPrompt: "summary", Temperature: .1, MaxTokens: 1000, MaxInputChars: 120000, TimeoutSeconds: 30, RetryCount: 1, BatchSize: 50}
	first, err := service.SaveConfig(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	input.BatchSize = 25
	second, err := service.SaveConfig(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || second.BatchSize != 25 {
		t.Fatalf("singleton update failed: first=%+v second=%+v", first, second)
	}
	var count int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM novel_chapter_summary_config`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}

func TestWorkerCompletesSummaryWithPostgreSQLAndMinIO(t *testing.T) {
	dsn := os.Getenv("MOONBOOK_CHAPTER_SUMMARY_TEST_DSN")
	endpoint := os.Getenv("MOONBOOK_CHAPTER_SUMMARY_TEST_MINIO_ENDPOINT")
	accessKey := os.Getenv("MOONBOOK_CHAPTER_SUMMARY_TEST_MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MOONBOOK_CHAPTER_SUMMARY_TEST_MINIO_SECRET_KEY")
	if dsn == "" || endpoint == "" || accessKey == "" || secretKey == "" {
		t.Skip("chapter summary PostgreSQL/MinIO test environment is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	bucket := fmt.Sprintf("moonbook-summary-test-%d", time.Now().UnixNano())
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, "")})
	if err != nil {
		t.Fatal(err)
	}
	if err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		for object := range client.ListObjects(cleanupCtx, bucket, minio.ListObjectsOptions{Recursive: true}) {
			if object.Err == nil {
				_ = client.RemoveObject(cleanupCtx, bucket, object.Key, minio.RemoveObjectOptions{})
			}
		}
		_ = client.RemoveBucket(cleanupCtx, bucket)
	})
	blobs, err := objectstore.NewMinIOStore(objectstore.MinIOConfig{Endpoint: endpoint, AccessKey: accessKey, SecretKey: secretKey, Bucket: bucket})
	if err != nil {
		t.Fatal(err)
	}
	objects := objectstore.NewService(db, blobs)
	suffix := time.Now().UnixNano()
	var categoryID, authorID, bookID, chapterID, cleanTaskID, cleanResultID, aiConfigID, modelID int64
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_categories(code,name,kind) VALUES($1,$2,'primary') RETURNING id`, fmt.Sprintf("summary-%d", suffix), "摘要测试分类").Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_authors(pen_name,normalized_name) VALUES($1,$1) RETURNING id`, fmt.Sprintf("摘要作者-%d", suffix)).Scan(&authorID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_books(primary_category_id,category_code,category_name,book_name,author_id,author_name) VALUES($1,$2,'摘要测试分类',$3,$4,$5) RETURNING id`, categoryID, fmt.Sprintf("summary-%d", suffix), fmt.Sprintf("摘要书-%d", suffix), authorID, fmt.Sprintf("摘要作者-%d", suffix)).Scan(&bookID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_chapters(book_id,chapter_no,chapter_name,word_count,ai_clean_status) VALUES($1,1,'第一章',8,'cleaned') RETURNING id`, bookID).Scan(&chapterID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_chapter_clean_task(book_id,book_name,status,total_count,processed_count,success_count) VALUES($1,$2,'completed',1,1,1) RETURNING id`, bookID, fmt.Sprintf("摘要书-%d", suffix)).Scan(&cleanTaskID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_chapter_clean_result(task_id,book_id,chapter_id,source_chapter_no,content_type,is_novel_body,cleaned_chapter_name,cleaned_word_count,status) VALUES($1,$2,$3,1,'novel',true,'第一章',8,'success') RETURNING id`, cleanTaskID, bookID, chapterID).Scan(&cleanResultID); err != nil {
		t.Fatal(err)
	}
	artifact, err := objects.UploadVerified(ctx, objectstore.Target{Kind: objectstore.KindChapterClean, BookID: bookID, OwnerID: cleanResultID}, []byte("第一段正文，主角发现一条密道。"), "text/plain; charset=utf-8")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE novel_chapter_clean_result SET cleaned_object_id=$2 WHERE id=$1`, cleanResultID, artifact.ID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_ai_config(config_name,base_url,stream_mode,failure_threshold,enabled) VALUES($1,'http://127.0.0.1:1','NON_STREAM',2,true) RETURNING id`, fmt.Sprintf("summary-worker-%d", suffix)).Scan(&aiConfigID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_ai_config_model(ai_config_id,model_name,sort_order) VALUES($1,'fixture',1) RETURNING id`, aiConfigID).Scan(&modelID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE novel_ai_config SET current_model_id=$2 WHERE id=$1`, aiConfigID, modelID); err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	_, err = service.SaveConfig(ctx, ConfigInput{AIConfigID: fmt.Sprint(aiConfigID), Enabled: false, SystemPrompt: "summary", Temperature: .1, MaxTokens: 1000, MaxInputChars: 8, TimeoutSeconds: 30, RetryCount: 1, BatchSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.Start(ctx, StartInput{OperatorName: "integration"})
	if err != nil {
		t.Fatal(err)
	}
	transport := &fixtureSummarizer{}
	worker := &Worker{DB: db, Jobs: service.Jobs, AI: aiconfig.NewService(db, nil), Objects: objects, Transport: transport, WorkerID: "summary-integration", Lease: time.Minute}
	worked := false
	for attempt := 0; attempt < 20 && !worked; attempt++ {
		worked, err = worker.RunOnce(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !worked {
			time.Sleep(25 * time.Millisecond)
		}
	}
	if !worked {
		t.Fatal("summary job was not claimable within 500ms")
	}
	var summary, taskStatus, jobStatus string
	var processed, succeeded int
	if err = db.QueryRowContext(ctx, `SELECT chapter_summary FROM novel_chapter_clean_result WHERE id=$1`, cleanResultID).Scan(&summary); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT status,processed_count,success_count FROM novel_chapter_summary_task WHERE id=$1`, task.ID).Scan(&taskStatus, &processed, &succeeded); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT status FROM platform_jobs WHERE module='novel' AND job_type='chapter_summary' AND idempotency_key=$1`, "chapter-summary:"+task.ID).Scan(&jobStatus); err != nil {
		t.Fatal(err)
	}
	if summary != "主角在密道中找到线索" || taskStatus != "completed" || processed != 1 || succeeded != 1 || jobStatus != "succeeded" {
		t.Fatalf("summary=%q task=%s processed=%d succeeded=%d job=%s", summary, taskStatus, processed, succeeded, jobStatus)
	}
	if transport.content != "第一段正文，主角" {
		t.Fatalf("maxInputChars was not applied: %q", transport.content)
	}
}
