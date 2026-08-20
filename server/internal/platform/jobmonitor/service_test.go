package jobmonitor

import (
	"context"
	"testing"
)

type fakeRepository struct{}

func (fakeRepository) List(context.Context, Filters, int, int) ([]Job, int64, error) {
	return []Job{{ID: "9007199254740993"}}, 1, nil
}
func (fakeRepository) Get(context.Context, string) (Detail, error) { return Detail{}, nil }

func TestServiceValidatesFiltersAndPreservesLongIDs(t *testing.T) {
	service := NewService(fakeRepository{})
	rows, total, err := service.List(context.Background(), Filters{Status: "running", From: "2026-08-20T00:00:00Z"}, 1, 20)
	if err != nil || total != 1 || len(rows) != 1 || rows[0].ID != "9007199254740993" {
		t.Fatalf("unexpected result rows=%v total=%d err=%v", rows, total, err)
	}
	if _, _, err := service.List(context.Background(), Filters{Status: "unknown"}, 1, 20); err == nil {
		t.Fatal("expected invalid status")
	}
	if _, _, err := service.List(context.Background(), Filters{From: "not-a-time"}, 1, 20); err == nil {
		t.Fatal("expected invalid time")
	}
}

func TestServiceRejectsInvalidIDsAndRedactsMessages(t *testing.T) {
	service := NewService(fakeRepository{})
	for _, id := range []string{"", "0", "-1", "abc", "9223372036854775808"} {
		if _, err := service.Get(context.Background(), id); err == nil {
			t.Fatalf("expected invalid ID %q", id)
		}
	}
	if got := sanitizeMessage("request failed api_key=top-secret"); got != "错误信息包含敏感字段，已脱敏" {
		t.Fatalf("unexpected redaction: %q", got)
	}
	if got := sanitizeMessage("line one\nline two"); got != "line one line two" {
		t.Fatalf("unexpected normalization: %q", got)
	}
	if got := sanitizeMessage("database postgres://reader:plain@example.test/moonbook"); got != "错误信息包含敏感字段，已脱敏" {
		t.Fatalf("unexpected DSN redaction: %q", got)
	}
}

func TestServiceRejectsReversedTimesAndOversizedFilters(t *testing.T) {
	service := NewService(fakeRepository{})
	if _, _, err := service.List(context.Background(), Filters{From: "2026-08-21T01:00:00Z", To: "2026-08-21T01:00:00Z"}, 1, 20); err == nil {
		t.Fatal("expected equal time bounds to fail")
	}
	if _, _, err := service.List(context.Background(), Filters{Module: string(make([]byte, maxFilterLength+1))}, 1, 20); err == nil {
		t.Fatal("expected oversized module filter to fail")
	}
}
