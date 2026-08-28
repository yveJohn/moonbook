//go:build integration

package adminpayment

import (
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/secretcrypto"
)

func TestPaymentChannelEncryptedConfigurationAndHealthCheck(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	health := &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})}}
	health.Start()
	defer health.Close()

	key := base64.StdEncoding.EncodeToString(bytesOf(17, 32))
	cipher, err := secretcrypto.New(key)
	if err != nil {
		t.Fatal(err)
	}
	repo := SQLRepository{DB: db, Cipher: cipher}
	input := ChannelInput{
		DisplayName: "EPUSDT 集成测试", Provider: "epusdt", Enabled: true, Currency: "usd", Token: "usdt", Network: "tron",
		MerchantPID: strPointer("integration-pid"), Secret: strPointer("integration-secret"),
		CreateURL: health.URL + "/create", NotifyURL: health.URL + "/notify", RedirectURL: health.URL + "/redirect",
		HealthURL: health.URL, SyncURL: health.URL + "/sync", ConnectTimeoutMS: 1000, RequestTimeoutMS: 2000, UnknownReleaseMinutes: 12,
	}

	var original struct {
		displayName, pidCipher, secretCipher, createURL, notifyURL, redirectURL, healthURL, syncURL string
		enabled                                                                                     bool
		connectMS, requestMS, unknownMinutes                                                        int
	}
	err = db.QueryRowContext(ctx, `SELECT display_name,enabled,merchant_pid_ciphertext,secret_ciphertext,create_url,notify_url,redirect_url,health_url,sync_url,connect_timeout_ms,request_timeout_ms,unknown_release_minutes FROM reader_payment_channels WHERE id=1`).Scan(
		&original.displayName, &original.enabled, &original.pidCipher, &original.secretCipher, &original.createURL, &original.notifyURL,
		&original.redirectURL, &original.healthURL, &original.syncURL, &original.connectMS, &original.requestMS, &original.unknownMinutes,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `UPDATE reader_payment_channels SET display_name=$1,enabled=$2,merchant_pid_ciphertext=$3,secret_ciphertext=$4,create_url=$5,notify_url=$6,redirect_url=$7,health_url=$8,sync_url=$9,connect_timeout_ms=$10,request_timeout_ms=$11,unknown_release_minutes=$12,archived_at=NULL WHERE id=1`,
			original.displayName, original.enabled, original.pidCipher, original.secretCipher, original.createURL, original.notifyURL, original.redirectURL,
			original.healthURL, original.syncURL, original.connectMS, original.requestMS, original.unknownMinutes)
	})

	updated, err := repo.Update(ctx, 1, input)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Configured() || !updated.Enabled || updated.ID != 1 {
		t.Fatalf("updated=%+v", updated)
	}
	var storedPID, storedSecret string
	if err = db.QueryRowContext(ctx, `SELECT merchant_pid_ciphertext,secret_ciphertext FROM reader_payment_channels WHERE id=1`).Scan(&storedPID, &storedSecret); err != nil {
		t.Fatal(err)
	}
	if storedPID == "integration-pid" || storedSecret == "integration-secret" || !strings.HasPrefix(storedPID, "v1:") || !strings.HasPrefix(storedSecret, "v1:") {
		t.Fatal("payment credentials were not stored as versioned ciphertext")
	}
	runtimeConfig, err := repo.Runtime(ctx)
	if err != nil {
		t.Fatal(err)
	}
	credential := runtimeConfig.EPUSDT.Credentials.Current()
	if credential.PID() != "integration-pid" || credential.Secret() != "integration-secret" || runtimeConfig.EPUSDT.UnknownReleaseWindow != 12*time.Minute {
		t.Fatal("runtime configuration did not decrypt the saved channel")
	}
	result, err := NewService(repo).Check(ctx, 1)
	if err != nil || result.Status != "reachable" || result.ChannelID != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func bytesOf(value byte, count int) []byte {
	out := make([]byte, count)
	for index := range out {
		out[index] = value
	}
	return out
}

func strPointer(value string) *string { return &value }
