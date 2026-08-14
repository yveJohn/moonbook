package runtimeconfig

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var templateVariables = []string{
	"POSTGRES_DB", "POSTGRES_USER", "POSTGRES_PASSWORD",
	"REDIS_PASSWORD", "MINIO_ROOT_USER", "MINIO_ROOT_PASSWORD", "MINIO_BUCKET",
	"MOONBOOK_JWT_SIGNING_KEY", "MOONBOOK_METRICS_TOKEN", "MOONBOOK_SERVER_PORT",
	"MOONBOOK_POSTGRES_HOST", "MOONBOOK_POSTGRES_PORT", "MOONBOOK_REDIS_ADDR",
	"MOONBOOK_MINIO_ENDPOINT", "MOONBOOK_MINIO_BUCKET_URL",
	"MOONBOOK_READER_ORIGIN",
}

type Lookup func(string) (string, bool)

func Render(templatePath, outputPath string, lookup Lookup) error {
	template, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("read config template: %w", err)
	}

	result := string(template)
	missing := make([]string, 0)
	for _, name := range templateVariables {
		value, ok := lookup(name)
		if !ok || value == "" {
			missing = append(missing, name)
			continue
		}
		encoded, err := yaml.Marshal(value)
		if err != nil {
			return fmt.Errorf("encode %s: %w", name, err)
		}
		result = strings.ReplaceAll(result, "${"+name+"}", strings.TrimSpace(string(encoded)))
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	if strings.Contains(result, "${") {
		return errors.New("rendered configuration contains unresolved variables")
	}

	var decoded any
	if err := yaml.Unmarshal([]byte(result), &decoded); err != nil {
		return fmt.Errorf("validate rendered configuration: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(result), 0o600); err != nil {
		return fmt.Errorf("write rendered configuration: %w", err)
	}
	if err := os.Chmod(outputPath, 0o600); err != nil {
		return fmt.Errorf("secure rendered configuration: %w", err)
	}
	return nil
}

func DatabaseDSN(lookup Lookup) (string, error) {
	required := []string{"POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB", "MOONBOOK_POSTGRES_HOST", "MOONBOOK_POSTGRES_PORT"}
	values := make(map[string]string, len(required))
	for _, name := range required {
		value, ok := lookup(name)
		if !ok || value == "" {
			return "", fmt.Errorf("missing required environment variable: %s", name)
		}
		values[name] = value
	}
	dsn := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(values["POSTGRES_USER"], values["POSTGRES_PASSWORD"]),
		Host:   values["MOONBOOK_POSTGRES_HOST"] + ":" + values["MOONBOOK_POSTGRES_PORT"],
		Path:   values["POSTGRES_DB"],
	}
	query := dsn.Query()
	query.Set("sslmode", "disable")
	dsn.RawQuery = query.Encode()
	return dsn.String(), nil
}
