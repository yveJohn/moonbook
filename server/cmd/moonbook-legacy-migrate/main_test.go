package main

import (
	"testing"
	"time"
)

func TestMigrationTimeout(t *testing.T) {
	if got, err := migrationTimeout(""); err != nil || got != 12*time.Hour {
		t.Fatalf("default timeout=%v err=%v", got, err)
	}
	if got, err := migrationTimeout("90m"); err != nil || got != 90*time.Minute {
		t.Fatalf("configured timeout=%v err=%v", got, err)
	}
	for _, value := range []string{"invalid", "30s", "25h"} {
		if _, err := migrationTimeout(value); err == nil {
			t.Fatalf("timeout %q should fail", value)
		}
	}
}
