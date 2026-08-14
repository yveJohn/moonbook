//go:build integration

package me

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

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
	service := NewService(SQLRepository{DB: db})
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
