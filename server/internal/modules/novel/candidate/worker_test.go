package candidate

import (
	"context"
	"testing"
)

func TestDiscoveryWorkerRunOnceRequiresConfiguration(t *testing.T) {
	worker := &DiscoveryWorker{}
	if _, err := worker.RunOnce(context.Background()); err == nil {
		t.Fatal("expected configuration error")
	} else if err.Error() != "candidate discovery worker is not configured" {
		t.Fatalf("unexpected error: %v", err)
	}
}
