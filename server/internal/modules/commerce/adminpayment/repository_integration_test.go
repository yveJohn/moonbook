//go:build integration

package adminpayment

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestCheckPaymentChannelUsesLocalHealthProbe(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})}}
	server.Start()
	defer server.Close()
	t.Setenv("MOONBOOK_EPUSDT_PID", "test-pid")
	t.Setenv("MOONBOOK_EPUSDT_SECRET", "test-secret")
	t.Setenv("MOONBOOK_EPUSDT_HEALTH_URL", server.URL)
	if _, err := db.ExecContext(ctx, `UPDATE reader_payment_channels SET enabled=true WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `UPDATE reader_payment_channels SET enabled=false WHERE id=1`)
	})
	v, err := (SQLRepository{DB: db}).Check(ctx, 1)
	if err != nil || v.Status != "reachable" || v.ChannelID != 1 || v.Provider != "epusdt" {
		t.Fatalf("v=%+v err=%v", v, err)
	}
}
