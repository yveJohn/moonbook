//go:build integration

package metrics

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestPaymentCallbackCollectorCountsOnlyStaleRuntimeReceived(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	collector := newPaymentCallbackCollector(db, 5*time.Minute)
	staleBefore := time.Now().UTC().Add(-5 * time.Minute)
	baseline, err := collector.queryStale(ctx, staleBefore)
	if err != nil {
		t.Fatal(err)
	}
	prefix := fmt.Sprintf("metrics-callback-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_payment_callback_logs WHERE request_id LIKE $1`, prefix+"%")
	})
	for _, row := range []struct {
		source, result string
		requestedAt    time.Time
	}{
		{source: "runtime", result: "received", requestedAt: time.Now().UTC().Add(-10 * time.Minute)},
		{source: "runtime", result: "received", requestedAt: time.Now().UTC()},
		{source: "manual", result: "received", requestedAt: time.Now().UTC().Add(-10 * time.Minute)},
	} {
		if _, err := db.ExecContext(ctx, `
INSERT INTO reader_payment_callback_logs
    (provider,payload_hash,signature_valid,processing_result,response_status,response_body,request_time,source_type,request_id)
VALUES ('epusdt',repeat('f',64),false,$1,0,'',$2,$3,$4)`, row.result, row.requestedAt, row.source, prefix+"-"+row.source+"-"+fmt.Sprint(row.requestedAt.UnixNano())); err != nil {
			t.Fatal(err)
		}
	}
	after, err := collector.queryStale(ctx, staleBefore)
	if err != nil {
		t.Fatal(err)
	}
	if after != baseline+1 {
		t.Fatalf("stale callbacks before=%v after=%v", baseline, after)
	}
}
