package initialize

import (
	"context"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/utils/logger"
)

func TestPaymentRequestMetadataUsesValidatedLoggerContext(t *testing.T) {
	ctx := logger.WithFields(context.Background(), &logger.Fields{RequestID: "request-validated", TraceID: "trace-validated", ClientIP: "203.0.113.40"})
	metadata := paymentRequestMetadata(ctx)
	if metadata.RequestID != "request-validated" || metadata.TraceID != "trace-validated" || metadata.ClientIP != "203.0.113.40" {
		t.Fatalf("metadata=%+v", metadata)
	}
	if empty := paymentRequestMetadata(context.Background()); empty.RequestID != "" || empty.TraceID != "" || empty.ClientIP != "" {
		t.Fatalf("empty metadata=%+v", empty)
	}
}
