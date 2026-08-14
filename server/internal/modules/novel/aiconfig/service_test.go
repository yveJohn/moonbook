package aiconfig

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestNormalizePreservesOrderedModelsAndLongIndependentSettings(t *testing.T) {
	input, err := normalize(Input{
		ConfigName: " OpenAI 主连接 ", BaseURL: "https://api.example.test/v1/", StreamMode: "stream",
		Models: []ModelInput{{ModelName: " model-a "}, {ModelName: "model-b"}}, FailureThreshold: "5",
		SecretEnvName: "MOONBOOK_AI_PRIMARY_API_KEY", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if input.ConfigName != "OpenAI 主连接" || input.BaseURL != "https://api.example.test/v1" || input.StreamMode != "STREAM" || input.failureThreshold != 5 {
		t.Fatalf("normalized input=%+v", input)
	}
	if len(input.modelNames) != 2 || input.modelNames[0] != "model-a" || input.modelNames[1] != "model-b" {
		t.Fatalf("model names=%v", input.modelNames)
	}
}

func TestNormalizeRejectsDuplicateModelsAndUnsafeSecretReference(t *testing.T) {
	base := Input{ConfigName: "AI", BaseURL: "https://api.example.test/v1", StreamMode: "AUTO", Models: []ModelInput{{ModelName: "same"}, {ModelName: "same"}}, FailureThreshold: "5", Enabled: true}
	if _, err := normalize(base); err == nil {
		t.Fatal("duplicate models must be rejected")
	}
	base.Models = []ModelInput{{ModelName: "model"}}
	base.SecretEnvName = "OPENAI_API_KEY"
	if _, err := normalize(base); err == nil {
		t.Fatal("unscoped secret environment reference must be rejected")
	}
}

func TestParseIDPreservesUnsafeJavaScriptLong(t *testing.T) {
	id, err := parseID("9007199254740993")
	if err != nil || id != 9007199254740993 {
		t.Fatalf("id=%d err=%v", id, err)
	}
}

func TestRuntimeConfigDoesNotSerializeOrFormatSecret(t *testing.T) {
	config := RuntimeConfig{ID: 1, ModelID: 2, apiKey: "must-not-leak"}
	encoded, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{string(encoded), fmt.Sprintf("%+v", config), fmt.Sprintf("%#v", config)} {
		if strings.Contains(output, "must-not-leak") {
			t.Fatalf("runtime secret leaked in %q", output)
		}
	}
}
