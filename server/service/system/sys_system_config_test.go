package system

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/config"
	model "github.com/flipped-aurora/gin-vue-admin/server/model/system"
)

func TestPublicSystemConfigDoesNotExposeSecrets(t *testing.T) {
	c := config.Server{
		JWT:     config.JWT{SigningKey: "jwt-secret", ExpiresTime: "24h"},
		Redis:   config.Redis{Addr: "redis:6379", Password: "redis-secret"},
		Email:   config.Email{Secret: "smtp-secret"},
		Minio:   config.Minio{AccessKeyId: "minio-key", AccessKeySecret: "minio-secret"},
		Metrics: config.Metrics{Token: "metrics-secret"},
		Pgsql:   config.Pgsql{GeneralDB: config.GeneralDB{Password: "db-secret", Path: "postgres"}},
	}
	b, err := json.Marshal(publicSystemConfig(c))
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	for _, secret := range []string{"jwt-secret", "redis-secret", "smtp-secret", "minio-key", "minio-secret", "metrics-secret", "db-secret"} {
		if strings.Contains(body, secret) {
			t.Fatalf("public config leaked secret %q: %s", secret, body)
		}
	}
	for _, marker := range []string{"signing-key-configured", "password-configured", "secret-configured", "token-configured"} {
		if !strings.Contains(body, marker) {
			t.Fatalf("public config missing marker %q: %s", marker, body)
		}
	}
}

func TestSetSystemConfigIsRejected(t *testing.T) {
	err := SystemConfigServiceApp.SetSystemConfig(nil, model.System{})
	if err == nil || !strings.Contains(err.Error(), "禁止") {
		t.Fatalf("expected read-only configuration rejection, got %v", err)
	}
}
