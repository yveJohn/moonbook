// Moonbook modification notice: configuration contract coverage updated, 2026-08-14.
package config

import (
	"bytes"
	"fmt"
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
				"MOONBOOK_MINIO_USE_SSL":   "false",
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

func TestApplicationMasterKeyEnvironmentContract(t *testing.T) {
	const envExamplePath = "../../.env.example"
	content, err := os.ReadFile(envExamplePath)
	if err != nil {
		t.Fatal(err)
	}
	values := parseEnvironmentExample(string(content))
	wantDefaults := map[string]string{"MOONBOOK_APP_MASTER_KEY": "", "MOONBOOK_EPUSDT_CALLBACK_STALE_MINUTES": "5"}
	for key, want := range wantDefaults {
		if got, exists := values[key]; !exists || got != want {
			t.Errorf("%s default=%q exists=%t, want %q", key, got, exists, want)
		}
	}

	composeContent, err := os.ReadFile("../../compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var compose struct {
		Services map[string]struct {
			Environment map[string]any `yaml:"environment"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(composeContent, &compose); err != nil {
		t.Fatal(err)
	}
	for _, serviceName := range []string{"migrate", "admin-bootstrap", "server"} {
		environment := compose.Services[serviceName].Environment
		for key := range wantDefaults {
			if _, exists := environment[key]; !exists {
				t.Errorf("service %s does not receive %s", serviceName, key)
			}
		}
	}
	value := fmt.Sprint(compose.Services["migrate"].Environment["MOONBOOK_APP_MASTER_KEY"])
	if !strings.Contains(value, "${MOONBOOK_APP_MASTER_KEY") {
		t.Errorf("compose hard-codes MOONBOOK_APP_MASTER_KEY instead of environment injection")
	}
	for _, removed := range []string{"MOONBOOK_EPUSDT_PID", "MOONBOOK_EPUSDT_SECRET", "MOONBOOK_EPUSDT_CREATE_URL", "MOONBOOK_EPUSDT_SYNC_URL"} {
		if _, exists := values[removed]; exists {
			t.Errorf("deprecated payment environment variable remains in example: %s", removed)
		}
		if _, exists := compose.Services["server"].Environment[removed]; exists {
			t.Errorf("deprecated payment environment variable remains in compose: %s", removed)
		}
	}
}

func TestForumCookieSecretEnvironmentContract(t *testing.T) {
	const key = "MOONBOOK_FORUM_COOKIE_EXAMPLE"
	content, err := os.ReadFile("../../.env.example")
	if err != nil {
		t.Fatal(err)
	}
	if value, exists := parseEnvironmentExample(string(content))[key]; !exists || value != "" {
		t.Fatalf("%s must be declared with an empty example value", key)
	}

	composeContent, err := os.ReadFile("../../compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var compose struct {
		Services map[string]struct {
			Environment map[string]any `yaml:"environment"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(composeContent, &compose); err != nil {
		t.Fatal(err)
	}
	for _, serviceName := range []string{"migrate", "admin-bootstrap", "server"} {
		value, exists := compose.Services[serviceName].Environment[key]
		if !exists || !strings.Contains(fmt.Sprint(value), "${"+key) {
			t.Errorf("service %s does not receive %s by environment injection", serviceName, key)
		}
	}
}

func parseEnvironmentExample(content string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok {
			values[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return values
}
