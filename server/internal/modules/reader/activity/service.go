package activity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const ApplicationTimeZone = "Asia/Kuala_Lumpur"

type Repository interface {
	Record(context.Context, int64, time.Time, time.Time) error
	Overview(context.Context, time.Time, *time.Location) (Overview, error)
}

type Service struct {
	repo Repository
	now  func() time.Time
	zone *time.Location
}

func NewService(repo Repository) *Service {
	zone, err := time.LoadLocation(ApplicationTimeZone)
	if err != nil {
		panic("load Moonbook application timezone: " + err.Error())
	}
	return &Service{repo: repo, now: time.Now, zone: zone}
}

func (s *Service) Record(ctx context.Context, readerID int64, at time.Time) error {
	if readerID <= 0 {
		return errors.New("reader id must be positive")
	}
	if at.IsZero() {
		at = s.now()
	}
	local := at.In(s.zone)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.zone)
	return s.repo.Record(ctx, readerID, day, at)
}

func (s *Service) Overview(ctx context.Context) (Overview, error) {
	return s.repo.Overview(ctx, s.now(), s.zone)
}

type SQLRepository struct{ DB *sql.DB }

func (r SQLRepository) Record(ctx context.Context, readerID int64, day, at time.Time) error {
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO reader_daily_activity(activity_date,reader_id,first_active_at)
		VALUES($1,$2,$3) ON CONFLICT(activity_date,reader_id) DO NOTHING`, day.Format(time.DateOnly), readerID, at)
	if err != nil {
		return err
	}
	_, err = r.DB.ExecContext(ctx, `INSERT INTO reader_operation_events(reader_id,event_type,event_name,detail,created_at) VALUES($1,'login','登录','读者登录成功',$2)`, readerID, at)
	return err
}

type Growth struct {
	TotalUserCount         int64 `json:"totalUserCount"`
	TodayNewUserCount      int64 `json:"todayNewUserCount"`
	YesterdayNewUserCount  int64 `json:"yesterdayNewUserCount"`
	Last7DaysNewUserCount  int64 `json:"last7DaysNewUserCount"`
	Last30DaysNewUserCount int64 `json:"last30DaysNewUserCount"`
}

type Activity struct {
	TodayActiveUserCount      int64  `json:"todayActiveUserCount"`
	YesterdayActiveUserCount  int64  `json:"yesterdayActiveUserCount"`
	Last7DaysActiveUserCount  int64  `json:"last7DaysActiveUserCount"`
	Last30DaysActiveUserCount int64  `json:"last30DaysActiveUserCount"`
	ActivityTrackingStartDate string `json:"activityTrackingStartDate"`
}

type DailyTrend struct {
	Date            string `json:"date"`
	NewUserCount    int64  `json:"newUserCount"`
	ActiveUserCount int64  `json:"activeUserCount"`
}

type Overview struct {
	Growth      Growth       `json:"growth"`
	Activity    Activity     `json:"activity"`
	DailyTrend  []DailyTrend `json:"dailyTrend"`
	GeneratedAt string       `json:"generatedAt"`
}

func (r SQLRepository) Overview(ctx context.Context, now time.Time, zone *time.Location) (Overview, error) {
	today := dateAt(now, zone)
	yesterday := today.AddDate(0, 0, -1)
	last7 := today.AddDate(0, 0, -6)
	last30 := today.AddDate(0, 0, -29)
	tomorrow := today.AddDate(0, 0, 1)
	var result Overview
	err := r.DB.QueryRowContext(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE created_at >= $1 AND created_at < $2),
		       count(*) FILTER (WHERE created_at >= $3 AND created_at < $1),
		       count(*) FILTER (WHERE created_at >= $4 AND created_at < $2),
		       count(*) FILTER (WHERE created_at >= $5 AND created_at < $2)
		FROM reader_accounts`, today, tomorrow, yesterday, last7, last30).Scan(
		&result.Growth.TotalUserCount, &result.Growth.TodayNewUserCount,
		&result.Growth.YesterdayNewUserCount, &result.Growth.Last7DaysNewUserCount,
		&result.Growth.Last30DaysNewUserCount)
	if err != nil {
		return Overview{}, fmt.Errorf("query reader growth: %w", err)
	}
	var tracking time.Time
	if err = r.DB.QueryRowContext(ctx, `SELECT tracking_start_date FROM reader_activity_settings WHERE id=1`).Scan(&tracking); err != nil {
		return Overview{}, fmt.Errorf("query activity tracking start: %w", err)
	}
	tracking = dateAt(tracking, zone)
	effective7, effective30 := laterDate(last7, tracking), laterDate(last30, tracking)
	err = r.DB.QueryRowContext(ctx, `
		SELECT count(*) FILTER (WHERE activity_date=$1),
		       count(*) FILTER (WHERE activity_date=$2),
		       count(DISTINCT reader_id) FILTER (WHERE activity_date BETWEEN $3 AND $1),
		       count(DISTINCT reader_id) FILTER (WHERE activity_date BETWEEN $4 AND $1)
		FROM reader_daily_activity WHERE activity_date BETWEEN $4 AND $1`, today, yesterday, effective7, effective30).Scan(
		&result.Activity.TodayActiveUserCount, &result.Activity.YesterdayActiveUserCount,
		&result.Activity.Last7DaysActiveUserCount, &result.Activity.Last30DaysActiveUserCount)
	if err != nil {
		return Overview{}, fmt.Errorf("query reader activity: %w", err)
	}
	if today.Before(tracking) {
		result.Activity.TodayActiveUserCount = 0
	}
	if yesterday.Before(tracking) {
		result.Activity.YesterdayActiveUserCount = 0
	}
	result.Activity.ActivityTrackingStartDate = tracking.Format(time.DateOnly)

	growth, err := countsByDate(ctx, r.DB, `
		SELECT (created_at AT TIME ZONE $1)::date, count(*)
		FROM reader_accounts WHERE created_at >= $2 AND created_at < $3
		GROUP BY 1 ORDER BY 1`, ApplicationTimeZone, last30, tomorrow)
	if err != nil {
		return Overview{}, fmt.Errorf("query daily growth: %w", err)
	}
	active, err := countsByDate(ctx, r.DB, `
		SELECT activity_date, count(*) FROM reader_daily_activity
		WHERE activity_date BETWEEN $1 AND $2 GROUP BY 1 ORDER BY 1`, laterDate(last30, tracking), today)
	if err != nil {
		return Overview{}, fmt.Errorf("query daily activity: %w", err)
	}
	result.DailyTrend = make([]DailyTrend, 0, 30)
	for day := last30; !day.After(today); day = day.AddDate(0, 0, 1) {
		key := day.Format(time.DateOnly)
		result.DailyTrend = append(result.DailyTrend, DailyTrend{Date: key, NewUserCount: growth[key], ActiveUserCount: active[key]})
	}
	result.GeneratedAt = now.In(zone).Format(time.RFC3339Nano)
	return result, nil
}

func countsByDate(ctx context.Context, db *sql.DB, query string, args ...any) (map[string]int64, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := make(map[string]int64)
	for rows.Next() {
		var day time.Time
		var count int64
		if err := rows.Scan(&day, &count); err != nil {
			return nil, err
		}
		counts[day.Format(time.DateOnly)] = count
	}
	return counts, rows.Err()
}

func dateAt(value time.Time, zone *time.Location) time.Time {
	local := value.In(zone)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, zone)
}

func laterDate(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
