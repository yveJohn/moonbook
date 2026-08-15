package me

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
	readerauth "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/auth"
	"github.com/gin-gonic/gin"
)

const (
	contractMaxID  int64 = 9223372036854775807
	contractSafeID int64 = 9007199254740993
)

var contractTime = time.Date(2026, 8, 15, 1, 2, 3, 0, time.UTC)

type contractRepo struct {
	feedbackPage int
	feedbackSize int
	chapterName  string
}

func (r *contractRepo) ListBookshelf(context.Context, int64) ([]Bookshelf, error) {
	r.chapterName = "最后一章"
	return []Bookshelf{{
		ID: contractMaxID, BookID: contractSafeID, BookName: "边界作品", AuthorName: "边界作者",
	}}, nil
}

func (r *contractRepo) GetBookshelf(_ context.Context, _ int64, bookID int64) (*Bookshelf, error) {
	r.chapterName = "最后一章"
	chapterID, readAt := contractMaxID-1, contractTime
	return &Bookshelf{ID: contractMaxID, BookID: bookID, LastChapterID: &chapterID, LastReadAt: &readAt}, nil
}

func (r *contractRepo) AddBookshelf(_ context.Context, _ int64, bookID int64) (Bookshelf, error) {
	chapterID, chapterName, readAt := contractMaxID-1, "最后一章", contractTime
	return Bookshelf{
		ID: contractMaxID, BookID: bookID, BookName: "边界作品", AuthorName: "边界作者",
		LastChapterID: &chapterID, LastChapterName: &chapterName, LastReadAt: &readAt,
	}, nil
}

func (r *contractRepo) RemoveBookshelf(context.Context, int64, int64) (bool, error) {
	return true, nil
}

func (r *contractRepo) ListLikes(context.Context, int64) ([]LikedBook, error) {
	return []LikedBook{{
		BookLikeID: contractMaxID, BookID: contractSafeID, BookName: "边界作品", AuthorName: "边界作者",
		Description: "简介", CategoryCode: "fantasy", CategoryName: "奇幻", WordCount: 123, LikeCount: 7,
		LikedAt: contractTime,
	}}, nil
}

func (r *contractRepo) Like(_ context.Context, _ int64, bookID int64) (BookLike, error) {
	return BookLike{ID: contractMaxID, BookID: bookID, Liked: true, LikeCount: 8}, nil
}

func (r *contractRepo) Unlike(_ context.Context, _ int64, bookID int64) (BookLike, error) {
	return BookLike{BookID: bookID, Liked: false, LikeCount: 7}, nil
}

func (r *contractRepo) CreateFeedback(_ context.Context, _ int64, content string) (Feedback, error) {
	return Feedback{ID: contractMaxID, Content: content, Status: "pending", CreatedAt: contractTime}, nil
}

func (r *contractRepo) ListFeedbacks(_ context.Context, _ int64, page, size int) ([]Feedback, int64, error) {
	r.feedbackPage, r.feedbackSize = page, size
	return []Feedback{}, 0, nil
}

func (r *contractRepo) ListHistory(context.Context, int64) ([]History, error) {
	r.chapterName = "第二章"
	return []History{{
		ID: contractMaxID, BookID: contractSafeID, ChapterID: contractMaxID - 1, ChapterNo: 2,
		ChapterName: "第二章", PositionType: "page", PositionValue: 3, ProgressPercent: "50.00",
		LastReadAt: contractTime,
	}}, nil
}

func (r *contractRepo) GetHistory(context.Context, int64, int64) (*History, error) {
	return nil, nil
}

func (r *contractRepo) UpdateHistory(_ context.Context, _ int64, bookID int64, input HistoryInput) (History, error) {
	return History{
		ID: contractMaxID, BookID: bookID, ChapterID: input.ChapterID, ChapterNo: 2,
		ChapterName: "第二章", PositionType: input.PositionType, PositionValue: input.PositionValue,
		ProgressPercent: input.ProgressPercent, LastReadAt: contractTime,
	}, nil
}

func (r *contractRepo) GetPreference(context.Context, int64) (*Preference, error) {
	return nil, nil
}

func (r *contractRepo) UpdatePreference(_ context.Context, _ int64, input PreferenceInput) (Preference, error) {
	return Preference{
		ID: contractMaxID, FontSize: *input.FontSize, LineHeight: input.LineHeight,
		Theme: input.Theme, ReadingMode: input.ReadingMode,
	}, nil
}

func TestReaderMeHTTPContractSnapshotsAndBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &contractRepo{}
	display := &fakeDisplay{fn: func(request novelcontract.DisplayRequest) novelcontract.DisplayBatch {
		batch := novelcontract.DisplayBatch{Books: make([]novelcontract.BookDisplay, len(request.BookIDs)), Chapters: make([]novelcontract.ChapterDisplay, len(request.ChapterIDs))}
		for index, id := range request.BookIDs {
			batch.Books[index] = novelcontract.BookDisplay{ID: id, Name: "边界作品", Author: "边界作者", Description: "简介", CategoryCode: "fantasy", CategoryName: "奇幻", WordCount: 123, LikeCount: 7, Found: true, Published: true}
		}
		for index, id := range request.ChapterIDs {
			name := "第二章"
			if id == contractMaxID-1 && repo.chapterName != "" {
				name = repo.chapterName
			}
			batch.Chapters[index] = novelcontract.ChapterDisplay{ID: id, BookID: contractSafeID, Number: 2, Name: name, Found: true, Enabled: true}
		}
		return batch
	}}
	likes := &fakeLikes{}
	handler := NewHandler(NewService(repo, display, immediateTransactor{}, likes, likes))
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("reader.identity", readerauth.Identity{ReaderID: contractMaxID, SessionID: contractSafeID})
		c.Next()
	})
	routes := router.Group("/reader/me")
	routes.GET("/bookshelf", handler.listBookshelf)
	routes.POST("/bookshelf/:bookId", handler.addBookshelf)
	routes.DELETE("/bookshelf/:bookId", handler.removeBookshelf)
	routes.GET("/likes", handler.listLikes)
	routes.POST("/likes/:bookId", handler.like)
	routes.DELETE("/likes/:bookId", handler.unlike)
	routes.POST("/feedbacks", handler.createFeedback)
	routes.GET("/feedbacks", handler.listFeedbacks)
	routes.GET("/history", handler.listHistory)
	routes.GET("/history/:bookId", handler.getHistory)
	routes.PUT("/history/:bookId", handler.updateHistory)
	routes.GET("/preference", handler.preference)
	routes.PUT("/preference", handler.updatePreference)

	tests := []struct {
		name, method, path, body, want string
	}{
		{"empty shelf fields", http.MethodGet, "/reader/me/bookshelf", "", `{"code":200,"msg":"查询成功","data":[{"bookshelfId":"9223372036854775807","bookId":"9007199254740993","bookName":"边界作品","authorName":"边界作者","lastChapterId":null,"lastChapterName":null,"lastReadTime":null}]}`},
		{"add shelf", http.MethodPost, "/reader/me/bookshelf/9007199254740993", "", `{"code":200,"msg":"操作成功","data":{"bookshelfId":"9223372036854775807","bookId":"9007199254740993","bookName":"边界作品","authorName":"边界作者","lastChapterId":"9223372036854775806","lastChapterName":"最后一章","lastReadTime":"2026-08-15 01:02:03"}}`},
		{"remove shelf", http.MethodDelete, "/reader/me/bookshelf/9223372036854775807", "", `{"code":200,"msg":"操作成功","data":true}`},
		{"list likes", http.MethodGet, "/reader/me/likes", "", `{"code":200,"msg":"查询成功","data":[{"likeId":"9223372036854775807","bookId":"9007199254740993","bookName":"边界作品","authorName":"边界作者","bookDesc":"简介","categoryCode":"fantasy","categoryName":"奇幻","wordCount":123,"likeCount":7,"likedAt":"2026-08-15 01:02:03"}]}`},
		{"like", http.MethodPost, "/reader/me/likes/9007199254740993", "", `{"code":200,"msg":"操作成功","data":{"likeId":"9223372036854775807","bookId":"9007199254740993","liked":true,"likeCount":8}}`},
		{"unlike nullable id", http.MethodDelete, "/reader/me/likes/9223372036854775807", "", `{"code":200,"msg":"操作成功","data":{"likeId":null,"bookId":"9223372036854775807","liked":false,"likeCount":7}}`},
		{"create feedback", http.MethodPost, "/reader/me/feedbacks", `{"content":"  契约反馈  "}`, `{"code":200,"msg":"操作成功","data":{"id":"9223372036854775807","content":"契约反馈","status":"pending","replyContent":null,"replyTime":null,"createTime":"2026-08-15 01:02:03"}}`},
		{"empty feedback page", http.MethodGet, "/reader/me/feedbacks?pageNum=0&pageSize=1000", "", `{"code":200,"msg":"查询成功","rows":[],"total":0}`},
		{"list history", http.MethodGet, "/reader/me/history", "", `{"code":200,"msg":"查询成功","data":[{"historyId":"9223372036854775807","bookId":"9007199254740993","chapterId":"9223372036854775806","chapterNo":2,"chapterName":"第二章","positionType":"page","positionValue":3,"progressPercent":"50.00","lastReadTime":"2026-08-15 01:02:03"}]}`},
		{"missing history is null", http.MethodGet, "/reader/me/history/9007199254740993", "", `{"code":200,"msg":"查询成功","data":null}`},
		{"update history", http.MethodPut, "/reader/me/history/9007199254740993", `{"chapterId":"9223372036854775807","chapterNo":2,"positionType":"page","positionValue":4,"progressPercent":"12.5"}`, `{"code":200,"msg":"操作成功","data":{"historyId":"9223372036854775807","bookId":"9007199254740993","chapterId":"9223372036854775807","chapterNo":2,"chapterName":"第二章","positionType":"page","positionValue":4,"progressPercent":"12.50","lastReadTime":"2026-08-15 01:02:03"}}`},
		{"default preference", http.MethodGet, "/reader/me/preference", "", `{"code":200,"msg":"查询成功","data":{"preferenceId":null,"fontSize":20,"lineHeight":"1.80","theme":"cream","readingMode":"page"}}`},
		{"normalized preference", http.MethodPut, "/reader/me/preference", `{"fontSize":99,"lineHeight":"1.401","theme":"invalid","readingMode":"invalid"}`, `{"code":200,"msg":"操作成功","data":{"preferenceId":"9223372036854775807","fontSize":28,"lineHeight":"1.40","theme":"cream","readingMode":"page"}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var body *bytes.Reader
			if test.body == "" {
				body = bytes.NewReader(nil)
			} else {
				body = bytes.NewReader([]byte(test.body))
			}
			req := httptest.NewRequest(test.method, test.path, body)
			if test.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
			}
			assertJSONEqual(t, test.want, resp.Body.String())
		})
	}
	if repo.feedbackPage != 1 || repo.feedbackSize != 100 {
		t.Fatalf("feedback pagination page=%d size=%d", repo.feedbackPage, repo.feedbackSize)
	}
}

func assertJSONEqual(t *testing.T, want, got string) {
	t.Helper()
	var wantValue, gotValue any
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("decode expected JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(got), &gotValue); err != nil {
		t.Fatalf("decode response JSON: %v; body=%s", err, got)
	}
	if !reflect.DeepEqual(wantValue, gotValue) {
		t.Fatalf("response mismatch\nwant: %s\n got: %s", want, got)
	}
}
