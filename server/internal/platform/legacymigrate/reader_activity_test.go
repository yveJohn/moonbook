package legacymigrate

import "testing"

func TestActivityCursor(t *testing.T) {
	id, day, err := activityCursor("")
	if err != nil || id != 0 || day != "0001-01-01" {
		t.Fatalf("empty cursor id=%d day=%s err=%v", id, day, err)
	}
	id, day, err = activityCursor("9007199254740993|2026-08-14")
	if err != nil || id != 9007199254740993 || day != "2026-08-14" {
		t.Fatalf("valid cursor id=%d day=%s err=%v", id, day, err)
	}
	for _, value := range []string{"reader_daily_activity:1", "1|2026-99-01", "-1|2026-08-14", "1|"} {
		if _, _, err := activityCursor(value); err == nil {
			t.Fatalf("cursor %q should be rejected", value)
		}
	}
}
