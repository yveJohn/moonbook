//go:build integration

package me

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	novelprovider "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/provider"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

type failingLikeSummary struct{ err error }

func (writer failingLikeSummary) SetLikeCount(context.Context, int64, int64) error {
	return writer.err
}

type swallowedNestedLikeSummary struct {
	tx  Transactor
	err error
}

func (writer swallowedNestedLikeSummary) SetLikeCount(ctx context.Context, _ int64, _ int64) error {
	_ = writer.tx.Within(ctx, func(context.Context) error { return writer.err })
	return nil
}

func TestReaderMePostgresReaderIsolation(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	reader1, reader2, category, author, book, chapter := base, base+1, base+2, base+3, base+4, base+5
	code := fmt.Sprintf("reader-me-it-%d", base)
	_, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash,status) VALUES($1,$2,'一号','x','enabled'),($3,$4,'二号','x','enabled')`, reader1, code+"-1", reader2, code+"-2")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO novel_categories(id,code,name,kind,source) VALUES($1,$2,$2,'primary','native')`, category, code)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO novel_authors(id,pen_name,normalized_name,status,source) VALUES($1,$2,$2,'active','native')`, author, code)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO novel_books(id,primary_category_id,category_code,category_name,book_name,author_id,author_name,publish_status,book_status,source_type,charge_mode) VALUES($1,$2,$3,$3,$3,$4,$3,'published','serializing','manual','login_free')`, book, category, code, author)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO novel_chapters(id,book_id,chapter_no,chapter_name,word_count,chapter_status,ai_clean_status,source_type) VALUES($1,$2,1,'第一章',4,'enabled','pending','manual')`, chapter, book)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup := context.Background()
		for _, q := range []string{`DELETE FROM reader_feedback WHERE reader_id IN ($1,$2)`, `DELETE FROM reader_reading_preferences WHERE reader_id IN ($1,$2)`, `DELETE FROM reader_reading_history WHERE reader_id IN ($1,$2)`, `DELETE FROM reader_book_likes WHERE reader_id IN ($1,$2)`, `DELETE FROM reader_bookshelf_entries WHERE reader_id IN ($1,$2)`, `DELETE FROM reader_sessions WHERE reader_id IN ($1,$2)`} {
			_, _ = db.ExecContext(cleanup, q, reader1, reader2)
		}
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_accounts WHERE id IN ($1,$2)`, reader1, reader2)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_chapters WHERE id=$1`, chapter)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_books WHERE id=$1`, book)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_authors WHERE id=$1`, author)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_categories WHERE id=$1`, category)
	})
	likesProvider := novelprovider.NewLikes(db)
	service := NewService(SQLRepository{DB: db}, novelprovider.NewDisplay(db), transaction.New(db), likesProvider, likesProvider)
	if _, err := service.AddBookshelf(ctx, reader1, book); err != nil {
		t.Fatalf("add bookshelf: %v", err)
	}
	if _, err := service.Like(ctx, reader1, book); err != nil {
		t.Fatalf("like: %v", err)
	}
	if _, err := service.UpdateHistory(ctx, reader1, book, HistoryInput{ChapterID: chapter, PositionType: "scroll", PositionValue: 12, ProgressPercent: "25"}); err != nil {
		t.Fatalf("history: %v", err)
	}
	font := 22
	if _, err := service.UpdatePreference(ctx, reader1, PreferenceInput{FontSize: &font, LineHeight: "1.90", Theme: "night", ReadingMode: "scroll"}); err != nil {
		t.Fatalf("preference: %v", err)
	}
	feedback, err := service.CreateFeedback(ctx, reader1, "正文测试反馈")
	if err != nil || feedback.Content != "正文测试反馈" {
		t.Fatalf("feedback=%+v err=%v", feedback, err)
	}
	shelf, err := service.ListBookshelf(ctx, reader1)
	if err != nil || len(shelf) != 1 || shelf[0].BookID != book {
		t.Fatalf("reader1 shelf=%+v err=%v", shelf, err)
	}
	likes, err := service.ListLikes(ctx, reader1)
	if err != nil || len(likes) != 1 || likes[0].BookID != book {
		t.Fatalf("reader1 likes=%+v err=%v", likes, err)
	}
	history, err := service.ListHistory(ctx, reader1)
	if err != nil || len(history) != 1 || history[0].ChapterID != chapter {
		t.Fatalf("reader1 history=%+v err=%v", history, err)
	}
	pref, err := service.GetPreference(ctx, reader1)
	if err != nil || pref.FontSize != 22 || pref.Theme != "night" {
		t.Fatalf("reader1 pref=%+v err=%v", pref, err)
	}
	otherShelf, _ := service.ListBookshelf(ctx, reader2)
	otherLikes, _ := service.ListLikes(ctx, reader2)
	otherHistory, _ := service.ListHistory(ctx, reader2)
	otherFeedback, total, _ := service.ListFeedbacks(ctx, reader2, 1, 10)
	if len(otherShelf) != 0 || len(otherLikes) != 0 || len(otherHistory) != 0 || len(otherFeedback) != 0 || total != 0 {
		t.Fatalf("reader data leaked across IDs: shelf=%v likes=%v history=%v feedback=%v/%d", otherShelf, otherLikes, otherHistory, otherFeedback, total)
	}
}

func TestReaderMeLikesAreAtomicIdempotentAndConcurrentWithPostgres(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	category, author, book, draftBook := base, base+1, base+2, base+3
	readers := make([]int64, 8)
	for index := range readers {
		readers[index] = base + 10 + int64(index)
	}
	code := fmt.Sprintf("reader-like-it-%d", base)
	t.Cleanup(func() {
		cleanup := context.Background()
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_book_likes WHERE book_id IN ($1,$2)`, book, draftBook)
		_, _ = db.ExecContext(cleanup, `DELETE FROM reader_accounts WHERE id >= $1 AND id <= $2`, readers[0], readers[len(readers)-1])
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_books WHERE id IN ($1,$2)`, book, draftBook)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_authors WHERE id=$1`, author)
		_, _ = db.ExecContext(cleanup, `DELETE FROM novel_categories WHERE id=$1`, category)
	})
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_categories(id,code,name,kind,source) VALUES($1,$2,$2,'primary','native')`, category, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_authors(id,pen_name,normalized_name,status,source) VALUES($1,$2,$2,'active','native')`, author, code); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO novel_books(id,primary_category_id,category_code,category_name,book_name,author_id,author_name,publish_status) VALUES($1,$2,$3,$3,$3,$4,$3,'published'),($5,$2,$3,$3,$6,$4,$3,'draft')`, book, category, code, author, draftBook, code+"-draft"); err != nil {
		t.Fatal(err)
	}
	for index, readerID := range readers {
		if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash,status) VALUES($1,$2,$2,'x','enabled')`, readerID, fmt.Sprintf("%s-%d", code, index)); err != nil {
			t.Fatal(err)
		}
	}
	tx := transaction.New(db)
	provider := novelprovider.NewLikes(db)
	service := NewService(SQLRepository{DB: db}, novelprovider.NewDisplay(db), tx, provider, provider)

	if _, err := service.Like(ctx, readers[0], draftBook); !errors.Is(err, ErrBookUnavailable) {
		t.Fatalf("draft like err=%v", err)
	}
	if _, err := service.Like(ctx, readers[0], base+999); !errors.Is(err, ErrBookUnavailable) {
		t.Fatalf("missing book like err=%v", err)
	}
	first, err := service.Like(ctx, readers[0], book)
	if err != nil || !first.Liked || first.LikeCount != 1 || first.ID == 0 {
		t.Fatalf("first like=%+v err=%v", first, err)
	}
	repeated, err := service.Like(ctx, readers[0], book)
	if err != nil || repeated.ID != first.ID || repeated.LikeCount != 1 {
		t.Fatalf("repeated like=%+v err=%v", repeated, err)
	}
	if _, err := service.Unlike(ctx, readers[0], book); err != nil {
		t.Fatalf("unlike: %v", err)
	}
	repeatedUnlike, err := service.Unlike(ctx, readers[0], book)
	if err != nil || repeatedUnlike.Liked || repeatedUnlike.LikeCount != 0 {
		t.Fatalf("repeated unlike=%+v err=%v", repeatedUnlike, err)
	}

	if _, err := service.Like(ctx, base+9999, book); !errors.Is(err, ErrBookUnavailable) {
		t.Fatalf("reader relation failure err=%v", err)
	}
	assertLikeFacts(t, ctx, db, book, 0)

	forced := errors.New("forced summary failure")
	failingService := NewService(SQLRepository{DB: db}, novelprovider.NewDisplay(db), tx, provider, failingLikeSummary{err: forced})
	if _, err := failingService.Like(ctx, readers[0], book); !errors.Is(err, ErrBookUnavailable) {
		t.Fatalf("summary failure err=%v", err)
	}
	assertLikeFacts(t, ctx, db, book, 0)

	swallowingService := NewService(SQLRepository{DB: db}, novelprovider.NewDisplay(db), tx, provider, swallowedNestedLikeSummary{tx: tx, err: forced})
	if _, err := swallowingService.Like(ctx, readers[0], book); !errors.Is(err, ErrBookUnavailable) {
		t.Fatalf("rollback-only err=%v", err)
	}
	assertLikeFacts(t, ctx, db, book, 0)

	runConcurrentLikes(t, ctx, readers, func(readerID int64) error {
		_, err := service.Like(ctx, readerID, book)
		return err
	})
	assertLikeFacts(t, ctx, db, book, int64(len(readers)))
	runConcurrentLikes(t, ctx, readers, func(readerID int64) error {
		_, err := service.Unlike(ctx, readerID, book)
		return err
	})
	assertLikeFacts(t, ctx, db, book, 0)
}

func runConcurrentLikes(t *testing.T, ctx context.Context, readerIDs []int64, operation func(int64) error) {
	t.Helper()
	errorsByReader := make(chan error, len(readerIDs))
	var workers sync.WaitGroup
	for _, readerID := range readerIDs {
		id := readerID
		workers.Add(1)
		go func() {
			defer workers.Done()
			errorsByReader <- operation(id)
		}()
	}
	workers.Wait()
	close(errorsByReader)
	for err := range errorsByReader {
		if err != nil {
			t.Fatalf("concurrent like operation: %v", err)
		}
	}
}

func assertLikeFacts(t *testing.T, ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, bookID, want int64) {
	t.Helper()
	var summary, relations int64
	if err := db.QueryRowContext(ctx, `SELECT like_count FROM novel_books WHERE id=$1`, bookID).Scan(&summary); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_book_likes WHERE book_id=$1`, bookID).Scan(&relations); err != nil {
		t.Fatal(err)
	}
	if summary != want || relations != want || summary != relations {
		t.Fatalf("like facts summary=%d relations=%d want=%d", summary, relations, want)
	}
}
