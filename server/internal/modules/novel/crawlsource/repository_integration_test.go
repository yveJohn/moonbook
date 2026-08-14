//go:build integration

package crawlsource

import (
	"context"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestForumSourceCRUDAndCookieRedaction(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r := SQLRepository{DB: db}
	v, err := r.Create(ctx, Input{SourceName: "集成论坛", BaseURL: "https://forum.example.test", RequestCharset: "UTF-8", CookieText: "session=secret", UserAgent: "Moonbook-Test", RequestIntervalMs: "1000", Enabled: true, SortOrder: "1", Remark: "fixture"})
	if err != nil || v.ID == "" || !v.CookieConfigured || v.BaseURL != "https://forum.example.test" {
		t.Fatalf("created=%+v err=%v", v, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM novel_crawl_forum_source WHERE id=$1`, v.ID)
	})
	items, total, err := r.List(ctx, "集成论坛", "true", 1, 20)
	if err != nil || total != 1 || len(items) != 1 || !items[0].CookieConfigured {
		t.Fatalf("items=%+v total=%d err=%v", items, total, err)
	}
	v, err = r.Update(ctx, parseID(v.ID), Input{SourceName: "集成论坛更新", BaseURL: "https://forum.example.test/v2", RequestCharset: "UTF-8", CookieText: "", UserAgent: "UA", RequestIntervalMs: "1500", Enabled: false, SortOrder: "2", Remark: "updated"})
	if err != nil || v.SourceName != "集成论坛更新" || !v.CookieConfigured || v.Enabled {
		t.Fatalf("updated=%+v err=%v", v, err)
	}
}

func parseID(v string) int64 {
	var n int64
	for _, r := range v {
		n = n*10 + int64(r-'0')
	}
	return n
}
