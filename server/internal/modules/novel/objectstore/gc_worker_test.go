package objectstore

import (
	"database/sql"
	"testing"
	"time"
)

func TestNewGCWorkerValidatesConfiguration(t *testing.T) {
	valid := GCWorkerConfig{WorkerID: "node-a", Interval: time.Hour, Lease: time.Minute, GracePeriod: 24 * time.Hour, BatchSize: 500, PollInterval: time.Second}
	if _, err := NewGCWorker(&sql.DB{}, &Service{}, valid); err != nil {
		t.Fatal(err)
	}
	invalid := valid
	invalid.BatchSize = 0
	if _, err := NewGCWorker(&sql.DB{}, &Service{}, invalid); err == nil {
		t.Fatal("invalid batch size should fail")
	}
}
