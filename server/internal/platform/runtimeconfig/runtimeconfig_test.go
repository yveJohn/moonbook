package runtimeconfig

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRenderEscapesEnvironmentValuesAndSecuresOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "config.yaml.tpl")
	outputPath := filepath.Join(dir, "run", "config.yaml")
	template := "password: ${POSTGRES_PASSWORD}\nendpoint: ${MOONBOOK_MINIO_BUCKET_URL}\n"
	if err := os.WriteFile(templatePath, []byte(template), 0o600); err != nil {
		t.Fatal(err)
	}
	values := completeValues()
	values["POSTGRES_PASSWORD"] = "p@ss: word #1\nsecond-line"
	values["MOONBOOK_MINIO_BUCKET_URL"] = "http://minio:9000/moonbook-content"

	if err := Render(templatePath, outputPath, mapLookup(values)); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	var rendered map[string]string
	if err := yaml.Unmarshal(content, &rendered); err != nil {
		t.Fatal(err)
	}
	if rendered["password"] != values["POSTGRES_PASSWORD"] {
		t.Fatalf("password changed during render: %q", rendered["password"])
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %o, want 600", got)
	}
}

func TestRenderRejectsMissingValues(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	templatePath := filepath.Join(dir, "config.yaml.tpl")
	if err := os.WriteFile(templatePath, []byte("password: ${POSTGRES_PASSWORD}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := completeValues()
	delete(values, "POSTGRES_PASSWORD")
	err := Render(templatePath, filepath.Join(dir, "config.yaml"), mapLookup(values))
	if err == nil || !strings.Contains(err.Error(), "POSTGRES_PASSWORD") {
		t.Fatalf("expected missing variable error, got %v", err)
	}
}

func TestDatabaseDSNRoundTripsSpecialCharacters(t *testing.T) {
	t.Parallel()
	values := completeValues()
	values["POSTGRES_USER"] = "moon@book"
	values["POSTGRES_PASSWORD"] = "p:/?#[]@ ss"
	values["POSTGRES_DB"] = "moon book"
	dsn, err := DatabaseDSN(mapLookup(values))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := parsed.User.Password()
	if parsed.User.Username() != values["POSTGRES_USER"] || password != values["POSTGRES_PASSWORD"] {
		t.Fatalf("credentials did not round trip: %s", dsn)
	}
	if strings.TrimPrefix(parsed.Path, "/") != "moon book" {
		t.Fatalf("database path = %q", parsed.Path)
	}
}

func completeValues() map[string]string {
	values := make(map[string]string, len(templateVariables))
	for _, name := range templateVariables {
		values[name] = "test-value"
	}
	values["MOONBOOK_POSTGRES_HOST"] = "postgres"
	values["MOONBOOK_POSTGRES_PORT"] = "5432"
	return values
}

func mapLookup(values map[string]string) Lookup {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}
