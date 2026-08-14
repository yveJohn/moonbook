package bookmerge

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestBookMergeWithPostgresAndMinIO(t *testing.T) {
	dsn := os.Getenv("MOONBOOK_BOOK_MERGE_TEST_DSN")
	endpoint := os.Getenv("MOONBOOK_BOOK_MERGE_TEST_MINIO_ENDPOINT")
	accessKey := os.Getenv("MOONBOOK_BOOK_MERGE_TEST_MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MOONBOOK_BOOK_MERGE_TEST_MINIO_SECRET_KEY")
	if dsn == "" || endpoint == "" || accessKey == "" || secretKey == "" {
		t.Skip("Moonbook book merge integration environment is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	bucket := fmt.Sprintf("moonbook-book-merge-%d", time.Now().UnixNano())
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, "")})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		for object := range client.ListObjects(cleanupCtx, bucket, minio.ListObjectsOptions{Recursive: true}) {
			if object.Err == nil {
				_ = client.RemoveObject(cleanupCtx, bucket, object.Key, minio.RemoveObjectOptions{})
			}
		}
		_ = client.RemoveBucket(cleanupCtx, bucket)
	})
	store, err := objectstore.NewMinIOStore(objectstore.MinIOConfig{Endpoint: endpoint, AccessKey: accessKey, SecretKey: secretKey, Bucket: bucket})
	if err != nil {
		t.Fatal(err)
	}
	objects := objectstore.NewService(db, store)
	service := NewService(db, objects)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	categoryCode := "merge-" + suffix
	var categoryID, authorID, sourceID, boardID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_categories(kind,code,name,sort,enabled) VALUES('primary',$1,$2,1,true) RETURNING id`, categoryCode, "合并分类"+suffix).Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_authors(pen_name,normalized_name,status) VALUES($1,$1,'active') RETURNING id`, "合并作者"+suffix).Scan(&authorID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_crawl_forum_source(source_name,base_url) VALUES($1,'https://example.test') RETURNING id`, "合并来源"+suffix).Scan(&sourceID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_crawl_forum_board(source_id,source_name,board_name,board_url) VALUES($1,$2,$3,'https://example.test/forum') RETURNING id`, sourceID, "合并来源"+suffix, "合并板块"+suffix).Scan(&boardID); err != nil {
		t.Fatal(err)
	}
	var bookOne, bookTwo int64
	for index, target := range []*int64{&bookOne, &bookTwo} {
		if err := db.QueryRowContext(ctx, `INSERT INTO novel_books(primary_category_id,category_code,category_name,book_name,author_id,author_name,book_status,publish_status,source_type,charge_mode) VALUES($1,$2,$3,$4,$5,$6,'completed','published','forum_crawl','login_free') RETURNING id`, categoryID, categoryCode, "合并分类"+suffix, fmt.Sprintf("源书%d-%s", index+1, suffix), authorID, "合并作者"+suffix).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	chapterIDs := make([]int64, 0, 3)
	fixtures := []struct {
		bookID  int64
		no      int
		name    string
		content string
	}{{bookOne, 1, "第一章", "相同正文"}, {bookOne, 2, "第二章", "需要排除"}, {bookTwo, 1, "续章", "相同正文"}}
	for _, fixture := range fixtures {
		var chapterID int64
		if err := db.QueryRowContext(ctx, `INSERT INTO novel_chapters(book_id,chapter_no,chapter_name,word_count,chapter_status,source_type) VALUES($1,$2,$3,$4,'enabled','forum_crawl') RETURNING id`, fixture.bookID, fixture.no, fixture.name, len([]rune(fixture.content))).Scan(&chapterID); err != nil {
			t.Fatal(err)
		}
		object, uploadErr := objects.UploadVerified(ctx, objectstore.Target{Kind: objectstore.KindChapterContent, BookID: fixture.bookID, OwnerID: chapterID}, []byte(fixture.content), "text/plain; charset=utf-8")
		if uploadErr != nil {
			t.Fatal(uploadErr)
		}
		if err := objects.Activate(ctx, object.ID); err != nil {
			t.Fatal(err)
		}
		chapterIDs = append(chapterIDs, chapterID)
	}
	for index, bookID := range []int64{bookOne, bookTwo} {
		threadID := fmt.Sprintf("thread-%d-%s", index+1, suffix)
		var candidateID int64
		if err := db.QueryRowContext(ctx, `INSERT INTO novel_crawl_thread_candidate(source_id,source_name,board_id,board_name,forum_thread_id,thread_title,thread_url,status,target_book_id) VALUES($1,$2,$3,$4,$5,$6,$7,'imported',$8) RETURNING id`, sourceID, "合并来源"+suffix, boardID, "合并板块"+suffix, threadID, "帖子"+threadID, "https://example.test/thread/"+threadID, bookID).Scan(&candidateID); err != nil {
			t.Fatal(err)
		}
		threadTime := "2026-08-12 10:00:00+00"
		if index == 0 {
			threadTime = "2026-08-13 10:00:00+00"
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO novel_crawl_import_task(candidate_id,source_id,source_name,board_id,board_name,forum_thread_id,thread_title,display_title,thread_url,target_book_id,import_mode,merge_strategy,status,thread_created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$7,$8,$9,'create','source_thread','succeeded',$10::timestamptz)`, candidateID, sourceID, "合并来源"+suffix, boardID, "合并板块"+suffix, threadID, "帖子"+threadID, "https://example.test/thread/"+threadID, bookID, threadTime); err != nil {
			t.Fatal(err)
		}
	}
	var missingObjectChapterID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_chapters(book_id,chapter_no,chapter_name,word_count,chapter_status,source_type) VALUES($1,99,'缺少正文对象',0,'enabled','forum_crawl') RETURNING id`, bookOne).Scan(&missingObjectChapterID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Preview(ctx, PreviewInput{SourceBookIDs: []string{strconv.FormatInt(bookOne, 10), strconv.FormatInt(bookTwo, 10)}}); err == nil {
		t.Fatal("source chapter without an active object must reject the merge preview")
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM novel_chapters WHERE id=$1`, missingObjectChapterID); err != nil {
		t.Fatal(err)
	}

	preview, err := service.Preview(ctx, PreviewInput{SourceBookIDs: []string{strconv.FormatInt(bookOne, 10), strconv.FormatInt(bookTwo, 10)}})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Sources) != 2 || preview.Sources[0].SourceBookID != strconv.FormatInt(bookTwo, 10) || preview.ChapterCount != "3" || preview.DuplicateChapterCount != "2" {
		t.Fatalf("preview=%+v", preview)
	}
	result, err := service.Execute(ctx, ExecuteInput{
		SourceBookIDs:      []string{strconv.FormatInt(bookOne, 10), strconv.FormatInt(bookTwo, 10)},
		ExcludedChapterIDs: []string{strconv.FormatInt(chapterIDs[1], 10)},
		TargetBook:         TargetBookInput{BookName: "合并目标" + suffix, AuthorID: strconv.FormatInt(authorID, 10), CategoryCode: categoryCode, BookStatus: "completed", PublishStatus: "published", OperatorName: "integration"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "succeeded" || result.IncludedChapterCount != "2" || result.ExcludedChapterCount != "1" || result.TargetBookID == "" {
		t.Fatalf("result=%+v", result)
	}
	targetBookID, _ := strconv.ParseInt(result.TargetBookID, 10, 64)
	var targetSource, sourceOneStatus, sourceTwoStatus string
	var targetChapterCount, lineageCount, activeObjectCount int
	if err := db.QueryRowContext(ctx, `SELECT source_type FROM novel_books WHERE id=$1`, targetBookID).Scan(&targetSource); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT publish_status FROM novel_books WHERE id=$1`, bookOne).Scan(&sourceOneStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT publish_status FROM novel_books WHERE id=$1`, bookTwo).Scan(&sourceTwoStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM novel_chapters WHERE book_id=$1 AND deleted_at IS NULL`, targetBookID).Scan(&targetChapterCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM novel_book_merge_chapter WHERE task_id=$1`, result.ID).Scan(&lineageCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM novel_object_references WHERE object_kind='chapter_content' AND book_id=$1`, targetBookID).Scan(&activeObjectCount); err != nil {
		t.Fatal(err)
	}
	if targetSource != "book_merge" || sourceOneStatus != "draft" || sourceTwoStatus != "draft" || targetChapterCount != 2 || lineageCount != 3 || activeObjectCount != 2 {
		t.Fatalf("targetSource=%s sourceStatus=%s/%s chapters=%d lineage=%d objects=%d", targetSource, sourceOneStatus, sourceTwoStatus, targetChapterCount, lineageCount, activeObjectCount)
	}
	if _, err := service.Execute(ctx, ExecuteInput{SourceBookIDs: []string{strconv.FormatInt(bookOne, 10), strconv.FormatInt(bookTwo, 10)}, TargetBook: TargetBookInput{BookName: "重复目标" + suffix, AuthorID: strconv.FormatInt(authorID, 10), CategoryCode: categoryCode, BookStatus: "completed", PublishStatus: "draft"}}); err == nil {
		t.Fatal("successful merge sources must not be merged again")
	}
}
