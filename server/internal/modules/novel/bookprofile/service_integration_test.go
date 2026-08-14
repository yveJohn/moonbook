package bookprofile

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/aiconfig"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/jobs"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type fixtureGenerator struct {
	input, categoryName, subCategoryName string
}

func (generator *fixtureGenerator) Generate(_ context.Context, _ aiconfig.RuntimeConfig, _ string, input string, _ float64, _ int, _ time.Duration) (SuggestedSnapshot, string, error) {
	generator.input = input
	return SuggestedSnapshot{BookName: "审核后的书名", BookDesc: "审核后的作品简介", CategoryName: generator.categoryName, SubCategories: []SnapshotCategory{{Name: generator.subCategoryName}}, Confidence: .96, Reason: "fixture"}, `{"fixture":true}`, nil
}

func TestWorkerApplyAndStaleSnapshotWithPostgreSQLAndMinIO(t *testing.T) {
	dsn := os.Getenv("MOONBOOK_BOOK_PROFILE_TEST_DSN")
	endpoint := os.Getenv("MOONBOOK_BOOK_PROFILE_TEST_MINIO_ENDPOINT")
	accessKey := os.Getenv("MOONBOOK_BOOK_PROFILE_TEST_MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MOONBOOK_BOOK_PROFILE_TEST_MINIO_SECRET_KEY")
	if dsn == "" || endpoint == "" || accessKey == "" || secretKey == "" {
		t.Skip("book profile PostgreSQL/MinIO test environment is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	bucket := fmt.Sprintf("moonbook-profile-test-%d", time.Now().UnixNano())
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
	var oldCategoryID, newCategoryID, subCategoryID, authorID, bookID, chapterID, cleanTaskID, cleanResultID, aiConfigID, modelID int64
	oldCode, newCode, subCode := fmt.Sprintf("profile-old-%d", suffix), fmt.Sprintf("profile-new-%d", suffix), fmt.Sprintf("profile-sub-%d", suffix)
	oldCategoryName, newCategoryName, subCategoryName := "旧主分类-"+fmt.Sprint(suffix), "新主分类-"+fmt.Sprint(suffix), "热血-"+fmt.Sprint(suffix)
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_categories(code,name,kind) VALUES($1,$2,'primary') RETURNING id`, oldCode, oldCategoryName).Scan(&oldCategoryID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_categories(code,name,kind) VALUES($1,$2,'primary') RETURNING id`, newCode, newCategoryName).Scan(&newCategoryID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_categories(code,name,kind) VALUES($1,$2,'sub') RETURNING id`, subCode, subCategoryName).Scan(&subCategoryID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_authors(pen_name,normalized_name) VALUES($1,$1) RETURNING id`, fmt.Sprintf("资料作者-%d", suffix)).Scan(&authorID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_books(primary_category_id,category_code,category_name,book_name,author_id,author_name,description) VALUES($1,$2,$3,$4,$5,$6,'旧简介') RETURNING id`, oldCategoryID, oldCode, oldCategoryName, fmt.Sprintf("资料书-%d", suffix), authorID, fmt.Sprintf("资料作者-%d", suffix)).Scan(&bookID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_chapters(book_id,chapter_no,chapter_name,word_count,ai_clean_status) VALUES($1,1,'第一章',10,'cleaned') RETURNING id`, bookID).Scan(&chapterID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_chapter_clean_task(book_id,book_name,status,total_count,processed_count,success_count) VALUES($1,'资料书','completed',1,1,1) RETURNING id`, bookID).Scan(&cleanTaskID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_chapter_clean_result(task_id,book_id,chapter_id,source_chapter_no,content_type,is_novel_body,cleaned_chapter_name,chapter_summary,cleaned_word_count,status) VALUES($1,$2,$3,1,'novel',true,'第一章','主角踏上旅程',10,'success') RETURNING id`, cleanTaskID, bookID, chapterID).Scan(&cleanResultID); err != nil {
		t.Fatal(err)
	}
	artifact, err := objects.UploadVerified(ctx, objectstore.Target{Kind: objectstore.KindChapterClean, BookID: bookID, OwnerID: cleanResultID}, []byte("主角在风雪中踏上旅程。"), "text/plain; charset=utf-8")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE novel_chapter_clean_result SET cleaned_object_id=$2 WHERE id=$1`, cleanResultID, artifact.ID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_ai_config(config_name,base_url,stream_mode,failure_threshold,enabled) VALUES($1,'http://fixture','NON_STREAM',2,true) RETURNING id`, fmt.Sprintf("profile-ai-%d", suffix)).Scan(&aiConfigID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `INSERT INTO novel_ai_config_model(ai_config_id,model_name,sort_order) VALUES($1,'fixture',1) RETURNING id`, aiConfigID).Scan(&modelID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE novel_ai_config SET current_model_id=$2 WHERE id=$1`, aiConfigID, modelID); err != nil {
		t.Fatal(err)
	}
	service := NewService(db, objects)
	if _, err = service.SaveConfig(ctx, Config{AIConfigID: fmt.Sprint(aiConfigID), SystemPrompt: "profile", Temperature: .1, MaxTokens: 1000, TimeoutSeconds: 30, RetryCount: 1, ScanBatchSize: 5, MaxInputChars: 60000}); err != nil {
		t.Fatal(err)
	}
	created, err := service.Generate(ctx, GenerateInput{BookID: fmt.Sprint(bookID)}, "manual")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE platform_jobs SET available_at='1900-01-01 00:00:00+00' WHERE module=$1 AND job_type=$2 AND idempotency_key=$3`, moduleName, jobType, "suggestion:"+created.ID); err != nil {
		t.Fatal(err)
	}
	deadJob, err := service.Jobs.Claim(ctx, jobs.ClaimOptions{WorkerID: "profile-integration-dead", Module: moduleName, Types: []string{jobType}, LeaseDuration: time.Minute})
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
	transport := &fixtureGenerator{categoryName: newCategoryName, subCategoryName: subCategoryName}
	worker := &Worker{DB: db, Jobs: service.Jobs, AI: aiconfig.NewService(db, nil), Service: service, Transport: transport, WorkerID: "profile-integration", Lease: time.Minute}
	worked := false
	for attempt := 0; attempt < 20 && !worked; attempt++ {
		worked, err = worker.RunOnce(ctx)
		if !worked && err == nil {
			time.Sleep(25 * time.Millisecond)
		}
	}
	if err != nil || !worked {
		t.Fatalf("worked=%t err=%v", worked, err)
	}
	pending, err := service.Get(ctx, mustID(t, created.ID), true)
	if err != nil || pending.Status != "pending" || pending.InputMode != "sampled_cleaned_text" || pending.Suggested == nil || pending.Suggested.CategoryCode != newCode {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
	if transport.input == "" {
		t.Fatal("worker did not receive the input snapshot")
	}
	var firstAttempts, firstRows int
	if err = db.QueryRowContext(ctx, `SELECT attempt_count FROM platform_jobs WHERE module=$1 AND job_type=$2 AND idempotency_key=$3`, moduleName, jobType, "suggestion:"+created.ID).Scan(&firstAttempts); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM novel_book_profile_suggestion WHERE id=$1`, created.ID).Scan(&firstRows); err != nil {
		t.Fatal(err)
	}
	if firstAttempts != 2 || firstRows != 1 {
		t.Fatalf("attempts=%d suggestion rows=%d", firstAttempts, firstRows)
	}
	if err = service.Apply(ctx, mustID(t, created.ID), ReviewInput{BookName: "审核后的书名", CategoryCode: newCode, BookDesc: "审核后的作品简介", SubCategoryCodes: []string{subCode}, ReviewerName: "integration"}); err != nil {
		t.Fatal(err)
	}
	var bookName, categoryCode, description string
	var relationCount int
	if err = db.QueryRowContext(ctx, `SELECT book_name,category_code,description FROM novel_books WHERE id=$1`, bookID).Scan(&bookName, &categoryCode, &description); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM novel_book_sub_categories WHERE book_id=$1 AND category_id=$2`, bookID, subCategoryID).Scan(&relationCount); err != nil {
		t.Fatal(err)
	}
	if bookName != "审核后的书名" || categoryCode != newCode || description != "审核后的作品简介" || relationCount != 1 {
		t.Fatalf("book=%q category=%q desc=%q relations=%d", bookName, categoryCode, description, relationCount)
	}
	stale, err := service.Generate(ctx, GenerateInput{BookID: fmt.Sprint(bookID)}, "manual")
	if err != nil {
		t.Fatal(err)
	}
	worked = false
	for attempt := 0; attempt < 20 && !worked; attempt++ {
		worked, err = worker.RunOnce(ctx)
		if !worked && err == nil {
			time.Sleep(25 * time.Millisecond)
		}
	}
	if err != nil || !worked {
		t.Fatalf("stale worker worked=%t err=%v", worked, err)
	}
	if _, err = db.ExecContext(ctx, `UPDATE novel_books SET description='外部修改',updated_at=now() WHERE id=$1`, bookID); err != nil {
		t.Fatal(err)
	}
	if err = service.Apply(ctx, mustID(t, stale.ID), ReviewInput{BookName: "审核后的书名", CategoryCode: newCode, BookDesc: "再次修改"}); err == nil {
		t.Fatal("stale suggestion was applied")
	}
}

func mustID(t *testing.T, value string) int64 {
	t.Helper()
	id, err := parseID(value, "测试 ID")
	if err != nil {
		t.Fatal(err)
	}
	return id
}
