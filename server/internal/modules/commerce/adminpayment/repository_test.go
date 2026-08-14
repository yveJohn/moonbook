package adminpayment

import (
	"context"
	"testing"
)

func TestProbeHealth(t *testing.T) {
	status, _ := probeHealth(context.Background(), "http://127.0.0.1:1")
	if status != "unreachable" {
		t.Fatalf("expected failed HTTP probe to be unreachable, got %s", status)
	}
	status, message := probeHealth(context.Background(), "://invalid")
	if status != "unreachable" || message != "健康检查请求失败" {
		t.Fatalf("status=%s message=%s", status, message)
	}
}
