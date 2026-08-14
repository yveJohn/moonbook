//go:build integration

package candidate

import (
	"context"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"strconv"
	"testing"
	"time"
)

func TestCandidateListAndStatusLifecycle(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var sourceID, boardID string
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_crawl_forum_source(source_name,base_url) VALUES('候选测试来源','https://candidate.example.test') RETURNING id::text`).Scan(&sourceID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_crawl_forum_board(source_id,source_name,board_name,board_url) VALUES($1,'候选测试来源','测试板块','https://candidate.example.test/forum') RETURNING id::text`, sourceID).Scan(&boardID); err != nil {
		t.Fatal(err)
	}
	var candidateID string
	if err := db.QueryRowContext(ctx, `INSERT INTO novel_crawl_thread_candidate(source_id,source_name,board_id,board_name,forum_thread_id,thread_title,thread_url) VALUES($1,'候选测试来源',$2,'测试板块','thread-1','测试帖子','https://candidate.example.test/thread-1') RETURNING id::text`, sourceID, boardID).Scan(&candidateID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM novel_crawl_thread_candidate WHERE id=$1`, candidateID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM novel_crawl_forum_board WHERE id=$1`, boardID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM novel_crawl_forum_source WHERE id=$1`, sourceID)
	})
	r := SQLRepository{DB: db}
	items, total, err := r.List(ctx, "测试帖子", sourceID, boardID, "pending", 1, 20)
	if err != nil || total != 1 || len(items) != 1 || items[0].ID != candidateID {
		t.Fatalf("items=%+v total=%d err=%v", items, total, err)
	}
	id, _ := strconv.ParseInt(candidateID, 10, 64)
	if err := r.Transition(ctx, []int64{id}, "skipped", ""); err != nil {
		t.Fatal(err)
	}
	v, err := r.Get(ctx, id)
	if err != nil || v.Status != "skipped" {
		t.Fatalf("candidate=%+v err=%v", v, err)
	}
	if err := r.Transition(ctx, []int64{id}, "pending", ""); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(ctx, []int64{id}); err != nil {
		t.Fatal(err)
	}
}
