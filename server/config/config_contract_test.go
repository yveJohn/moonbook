// Moonbook modification notice: configuration contract coverage updated, 2026-08-14.
package config

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestShippedServerConfigsMatchSchema(t *testing.T) {
	tests := []struct {
		name string
		path string
		env  map[string]string
	}{
		{name: "default", path: "../config.yaml"},
		{name: "docker", path: "../config.docker.yaml"},
		{
			name: "moonbook",
			path: "../config.moonbook.yaml.tpl",
			env: map[string]string{
				"MOONBOOK_JWT_SIGNING_KEY": "test-jwt-signing-key",
				"MOONBOOK_SERVER_PORT":     "18888",
				"MOONBOOK_METRICS_TOKEN":   "test-metrics-token",
				"POSTGRES_HOST_PORT":       "15432",
				"POSTGRES_DB":              "moonbook",
				"POSTGRES_USER":            "moonbook",
				"POSTGRES_PASSWORD":        "test-postgres-password",
				"REDIS_HOST_PORT":          "16379",
				"REDIS_PASSWORD":           "test-redis-password",
				"MINIO_API_HOST_PORT":      "19000",
				"MINIO_ROOT_USER":          "moonbook",
				"MINIO_ROOT_PASSWORD":      "test-minio-password",
				"MINIO_BUCKET":             "moonbook-content",
				"MOONBOOK_ADMIN_ORIGIN":    "http://localhost:8080",
				"MOONBOOK_READER_ORIGIN":   "http://localhost:8081",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			if len(tt.env) > 0 {
				content = []byte(os.Expand(string(content), func(key string) string {
					return tt.env[key]
				}))
				if strings.Contains(string(content), "${") {
					t.Fatal("配置模板渲染后仍包含未解析变量")
				}
			}

			decoder := yaml.NewDecoder(bytes.NewReader(content))
			decoder.KnownFields(true)
			var cfg Server
			if err := decoder.Decode(&cfg); err != nil {
				t.Fatalf("配置模板与 config.Server 不一致: %v", err)
			}
			if tt.name == "moonbook" {
				origins := make(map[string]bool, len(cfg.Cors.Whitelist))
				for _, rule := range cfg.Cors.Whitelist {
					origins[rule.AllowOrigin] = true
				}
				for _, origin := range []string{tt.env["MOONBOOK_ADMIN_ORIGIN"], tt.env["MOONBOOK_READER_ORIGIN"]} {
					if !origins[origin] {
						t.Errorf("CORS 白名单缺少运行时 Origin %s", origin)
					}
				}
			}
		})
	}
}
