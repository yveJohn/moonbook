package legacymigrate

import "testing"

func TestValidateLegacyAIConfigDoesNotExposeSecret(t *testing.T) {
	item := legacyAIConfig{id: 1, name: "清洗", baseURL: "https://ai.example.test/v1", failureThreshold: 5, enabled: 1, apiKey: "do-not-log"}
	if code, message := validateLegacyAIConfig(item); code != "" || message != "" {
		t.Fatalf("valid AI config rejected: %s %s", code, message)
	}
	ref := "MOONBOOK_AI_LEGACY_1_API_KEY"
	if !legacyAISecretPattern.MatchString(ref) {
		t.Fatalf("secret reference %q rejected", ref)
	}
	if ref == item.apiKey {
		t.Fatal("secret value was reused as reference")
	}
}

func TestValidateLegacyAIModel(t *testing.T) {
	valid := legacyAIModel{id: 10, configID: 1, name: "model-a", sortOrder: 1}
	if code, _ := validateLegacyAIModel(valid, 1); code != "" {
		t.Fatalf("valid model code=%s", code)
	}
	if code, _ := validateLegacyAIModel(valid, 2); code != "INVALID_AI_MODEL" {
		t.Fatalf("owner mismatch code=%s", code)
	}
}
