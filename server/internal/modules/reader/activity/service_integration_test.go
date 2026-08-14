//go:build integration

package activity

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestActivityRecordingAndDashboardOverview(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var firstID int64
	if err := db.QueryRowContext(ctx, `SELECT GREATEST(COALESCE(max(id),0)+10,9007199254745000) FROM reader_accounts`).Scan(&firstID); err != nil {
		t.Fatal(err)
	}
	ids := []int64{firstID, firstID + 1, firstID + 2}
	var originalTracking time.Time
	if err := db.QueryRowContext(ctx, `SELECT tracking_start_date FROM reader_activity_settings WHERE id=1`).Scan(&originalTracking); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_daily_activity WHERE reader_id=ANY($1)`, ids)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=ANY($1)`, ids)
		_, _ = db.ExecContext(context.Background(), `UPDATE reader_activity_settings SET tracking_start_date=$1 WHERE id=1`, originalTracking)
	})
	for index, created := range []string{"2035-01-01T12:00:00+08:00", "2035-01-25T12:00:00+08:00", "2035-01-29T12:00:00+08:00"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,created_at,updated_at) VALUES($1,$2,'fixture',$3,$3)`, ids[index], fmt.Sprintf("activity-it-%d", ids[index]), created); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `UPDATE reader_activity_settings SET tracking_start_date='2035-01-10' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	service := NewService(SQLRepository{DB: db})
	service.now = func() time.Time { return time.Date(2035, 1, 30, 4, 0, 0, 0, time.UTC) }
	records := []struct {
		reader int64
		at     time.Time
	}{
		{ids[0], time.Date(2035, 1, 29, 15, 59, 58, 0, time.UTC)},
		{ids[0], time.Date(2035, 1, 29, 16, 0, 2, 0, time.UTC)},
		{ids[0], time.Date(2035, 1, 29, 16, 5, 0, 0, time.UTC)},
		{ids[1], time.Date(2035, 1, 25, 1, 0, 0, 0, time.UTC)},
		{ids[1], time.Date(2035, 1, 30, 1, 0, 0, 0, time.UTC)},
	}
	for _, item := range records {
		if err := service.Record(ctx, item.reader, item.at); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	var first time.Time
	if err := db.QueryRowContext(ctx, `SELECT count(*),min(first_active_at) FROM reader_daily_activity WHERE reader_id=$1 AND activity_date='2035-01-30'`, ids[0]).Scan(&count, &first); err != nil {
		t.Fatal(err)
	}
	if count != 1 || !first.Equal(records[1].at) {
		t.Fatalf("daily dedup count=%d first=%s", count, first)
	}

	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_accounts WHERE id <> ALL($1)`, ids).Scan(&count); err != nil {
		t.Fatal(err)
	}
	baseTotal := int64(count)
	overview, err := service.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if overview.Growth.TotalUserCount != baseTotal+3 || overview.Growth.TodayNewUserCount != 0 || overview.Growth.YesterdayNewUserCount != 1 || overview.Growth.Last7DaysNewUserCount != 2 || overview.Growth.Last30DaysNewUserCount != 3 {
		t.Fatalf("growth=%+v base=%d", overview.Growth, baseTotal)
	}
	if overview.Activity.TodayActiveUserCount != 2 || overview.Activity.YesterdayActiveUserCount != 1 || overview.Activity.Last7DaysActiveUserCount != 2 || overview.Activity.Last30DaysActiveUserCount != 2 || overview.Activity.ActivityTrackingStartDate != "2035-01-10" {
		t.Fatalf("activity=%+v", overview.Activity)
	}
	if len(overview.DailyTrend) != 30 || overview.DailyTrend[0].Date != "2035-01-01" || overview.DailyTrend[29].Date != "2035-01-30" {
		t.Fatalf("trend boundaries=%+v", overview.DailyTrend)
	}
	if overview.DailyTrend[0].NewUserCount != 1 || overview.DailyTrend[0].ActiveUserCount != 0 || overview.DailyTrend[29].ActiveUserCount != 2 {
		t.Fatalf("trend values first=%+v last=%+v", overview.DailyTrend[0], overview.DailyTrend[29])
	}
}
