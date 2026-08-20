package metrics

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestOperationalAuditRefreshCachesSuccessfulSnapshot(t *testing.T) {
	collector := newOperationalAuditCollector(nil)
	collector.audit = func(context.Context) (operationalAuditSnapshot, error) {
		return operationalAuditSnapshot{migrationErrors: 2, unhealthyObjects: 3, financeMismatches: 4}, nil
	}
	now := time.Unix(1234, 0).UTC()
	collector.refresh(context.Background(), now)
	if collector.snapshot.migrationErrors != 2 || collector.snapshot.unhealthyObjects != 3 || collector.snapshot.financeMismatches != 4 {
		t.Fatalf("snapshot=%+v", collector.snapshot)
	}
	if collector.snapshot.collectionSuccess != 1 || collector.snapshot.lastSuccess != 1234 {
		t.Fatalf("status snapshot=%+v", collector.snapshot)
	}
}

func TestOperationalAuditFailurePreservesValuesAndMarksCollectionFailed(t *testing.T) {
	collector := newOperationalAuditCollector(nil)
	collector.snapshot = operationalAuditSnapshot{migrationErrors: 2, unhealthyObjects: 3, financeMismatches: 4, collectionSuccess: 1, lastSuccess: 1234}
	collector.audit = func(context.Context) (operationalAuditSnapshot, error) {
		return operationalAuditSnapshot{}, errors.New("audit unavailable")
	}
	collector.refresh(context.Background(), time.Unix(5678, 0))
	if collector.snapshot.collectionSuccess != 0 || collector.snapshot.lastSuccess != 1234 {
		t.Fatalf("status snapshot=%+v", collector.snapshot)
	}
	if collector.snapshot.migrationErrors != 2 || collector.snapshot.unhealthyObjects != 3 || collector.snapshot.financeMismatches != 4 {
		t.Fatalf("cached values changed: %+v", collector.snapshot)
	}
}
