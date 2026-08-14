//go:build integration

package adminfeedback

import (
	"context"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"testing"
	"time"
)

func TestAdminFeedbackReplyIsSingleTransition(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var rid int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),2607300000000)+1 FROM reader_accounts`).Scan(&rid); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash) VALUES($1,'admin-feedback-fixture','fixture')`, rid); err != nil {
		t.Fatal(err)
	}
	var fid int64
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_feedback(reader_id,content) VALUES($1,'反馈内容') RETURNING id`, rid).Scan(&fid); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_feedback WHERE id=$1`, fid)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, rid)
	})
	r := SQLRepository{DB: db}
	v, err := r.Reply(ctx, fid, "已处理")
	if err != nil || v.Status != "replied" || v.Reply != "已处理" {
		t.Fatalf("v=%+v err=%v", v, err)
	}
	if _, err = r.Reply(ctx, fid, "覆盖"); err == nil {
		t.Fatal("second reply unexpectedly succeeded")
	}
}
