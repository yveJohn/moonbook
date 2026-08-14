package wire

import (
	"testing"
	"time"
)

func TestDateTimeMatchesFrozenReaderFormatAndNulls(t *testing.T) {
	value := time.Date(2026, 8, 15, 14, 5, 6, 0, time.FixedZone("UTC+8", 8*60*60))
	if got := DateTime(value); got != "2026-08-15 06:05:06" {
		t.Fatalf("date time=%v", got)
	}
	if DateTime(time.Time{}) != nil || DateTimePointer(nil) != nil {
		t.Fatal("zero and nil times must remain null")
	}
}
