package epusdt

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigDisabledDoesNotRequirePaymentEnvironment(t *testing.T) {
	config, err := LoadConfig(false, mapLookup(nil))
	if err != nil {
		t.Fatal(err)
	}
	if config.Enabled || config.Credentials != nil {
		t.Fatalf("disabled config=%+v", config)
	}
}

func TestLoadConfigEnabled(t *testing.T) {
	env := validEnvironment()
	config, err := LoadConfig(true, mapLookup(env))
	if err != nil {
		t.Fatal(err)
	}
	if !config.Enabled || config.CreateURL.String() != env[EnvCreateURL] || config.NotifyURL.String() != env[EnvNotifyURL] || config.RedirectURL.String() != env[EnvRedirectURL] {
		t.Fatalf("config URLs=%+v", config)
	}
	if config.ConnectTimeout != 3*time.Second || config.RequestTimeout != 10*time.Second || config.UnknownReleaseWindow != 15*time.Minute {
		t.Fatalf("config durations=%+v", config)
	}
	current := config.Credentials.Current()
	if current.Ref() != "primary" || current.PID() != "merchant-current" || current.Secret() != "current-secret" {
		t.Fatalf("current credential ref=%q pid=%q", current.Ref(), current.PID())
	}
	historical, ok := config.Credentials.Verification("previous-2026-07")
	if !ok || historical.PID() != "merchant-previous" || historical.Secret() != "previous-secret" {
		t.Fatalf("historical credential found=%t ref=%q", ok, historical.Ref())
	}
	legacy, ok := config.Credentials.Verification("")
	if !ok || legacy.Ref() != current.Ref() {
		t.Fatalf("legacy credential found=%t ref=%q", ok, legacy.Ref())
	}
}

func TestLoadConfigEnabledRequiresCompleteEnvironment(t *testing.T) {
	for _, key := range []string{EnvPID, EnvSecret, EnvCredentialRef, EnvCreateURL, EnvNotifyURL, EnvRedirectURL} {
		t.Run(key, func(t *testing.T) {
			env := validEnvironment()
			delete(env, key)
			if _, err := LoadConfig(true, mapLookup(env)); err == nil || !strings.Contains(err.Error(), key) {
				t.Fatalf("missing %s error=%v", key, err)
			}
		})
	}
}

func TestLoadConfigRejectsInvalidURLs(t *testing.T) {
	for _, rawURL := range []string{
		"relative/path",
		"ftp://gateway.example/create",
		"https://user:pass@gateway.example/create",
		"https://gateway.example/create#fragment",
		"https:///missing-host",
	} {
		t.Run(rawURL, func(t *testing.T) {
			env := validEnvironment()
			env[EnvCreateURL] = rawURL
			if _, err := LoadConfig(true, mapLookup(env)); err == nil || !strings.Contains(err.Error(), EnvCreateURL) {
				t.Fatalf("URL %q error=%v", rawURL, err)
			}
		})
	}
}

func TestLoadConfigRejectsInvalidDurations(t *testing.T) {
	tests := []struct {
		key, value string
	}{
		{EnvConnectTimeoutMS, "99"},
		{EnvConnectTimeoutMS, "30001"},
		{EnvRequestTimeoutMS, "99"},
		{EnvRequestTimeoutMS, "120001"},
		{EnvUnknownReleaseMinutes, "0"},
		{EnvUnknownReleaseMinutes, "1441"},
		{EnvRequestTimeoutMS, "not-a-number"},
	}
	for _, test := range tests {
		t.Run(test.key+"="+test.value, func(t *testing.T) {
			env := validEnvironment()
			env[test.key] = test.value
			if _, err := LoadConfig(true, mapLookup(env)); err == nil || !strings.Contains(err.Error(), test.key) {
				t.Fatalf("duration %s=%q error=%v", test.key, test.value, err)
			}
		})
	}

	env := validEnvironment()
	env[EnvConnectTimeoutMS] = "5000"
	env[EnvRequestTimeoutMS] = "4999"
	if _, err := LoadConfig(true, mapLookup(env)); err == nil || !strings.Contains(err.Error(), EnvRequestTimeoutMS) {
		t.Fatalf("request below connect error=%v", err)
	}
}

func TestCredentialProviderRejectsInvalidCredentials(t *testing.T) {
	tests := []struct {
		name, currentRef, pid, secret, history string
	}{
		{name: "empty current ref", pid: "pid", secret: "secret", history: "[]"},
		{name: "invalid current ref", currentRef: "not valid", pid: "pid", secret: "secret", history: "[]"},
		{name: "empty current pid", currentRef: "primary", secret: "secret", history: "[]"},
		{name: "empty current secret", currentRef: "primary", pid: "pid", history: "[]"},
		{name: "invalid json", currentRef: "primary", pid: "pid", secret: "secret", history: "not-json"},
		{name: "object instead of array", currentRef: "primary", pid: "pid", secret: "secret", history: `{}`},
		{name: "unknown field", currentRef: "primary", pid: "pid", secret: "secret", history: `[{"ref":"old","pid":"old-pid","secret":"old-secret","extra":true}]`},
		{name: "empty historical field", currentRef: "primary", pid: "pid", secret: "secret", history: `[{"ref":"old","pid":"","secret":"old-secret"}]`},
		{name: "duplicate current ref", currentRef: "primary", pid: "pid", secret: "secret", history: `[{"ref":"primary","pid":"old-pid","secret":"old-secret"}]`},
		{name: "duplicate historical ref", currentRef: "primary", pid: "pid", secret: "secret", history: `[{"ref":"old","pid":"old-pid","secret":"one"},{"ref":"old","pid":"other-pid","secret":"two"}]`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewCredentialProvider(test.currentRef, test.pid, test.secret, test.history); err == nil {
				t.Fatal("expected credential validation error")
			}
		})
	}
}

func TestCredentialErrorsAndFormattingDoNotLeakSecrets(t *testing.T) {
	const (
		pid     = "merchant-sensitive-pid"
		secret  = "sensitive-current-secret"
		history = `[{"ref":"primary","pid":"historical-sensitive-pid","secret":"historical-sensitive-secret"}]`
	)
	provider, err := NewCredentialProvider("primary", pid, secret, "[]")
	if err != nil {
		t.Fatal(err)
	}
	formatted := fmt.Sprintf("provider=%v/%+v/%#v current=%v/%+v/%#v", provider, provider, provider, provider.Current(), provider.Current(), provider.Current())
	for _, sensitive := range []string{pid, secret} {
		if strings.Contains(formatted, sensitive) {
			t.Fatalf("formatting leaked sensitive value %q", sensitive)
		}
	}
	_, err = NewCredentialProvider("primary", pid, secret, history)
	if err == nil {
		t.Fatal("expected duplicate reference error")
	}
	for _, sensitive := range []string{pid, secret, history, "historical-sensitive-pid", "historical-sensitive-secret"} {
		if strings.Contains(err.Error(), sensitive) {
			t.Fatalf("error leaked sensitive value %q: %v", sensitive, err)
		}
	}
}

func TestVerificationDoesNotFallBackForUnknownReference(t *testing.T) {
	provider, err := NewCredentialProvider("primary", "pid", "secret", `[{"ref":"old","pid":"old-pid","secret":"old-secret"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := provider.Verification("missing"); ok {
		t.Fatal("unknown reference fell back to another credential")
	}
}

func validEnvironment() map[string]string {
	return map[string]string{
		EnvPID:                   "merchant-current",
		EnvSecret:                "current-secret",
		EnvCredentialRef:         "primary",
		EnvVerifyCredentialsJSON: `[{"ref":"previous-2026-07","pid":"merchant-previous","secret":"previous-secret"}]`,
		EnvCreateURL:             "https://gateway.example/api/v1/order/create-transaction",
		EnvNotifyURL:             "https://reader.example/prod-api/reader/payment/callback/epusdt",
		EnvRedirectURL:           "https://reader.example/recharge/result",
		EnvConnectTimeoutMS:      "3000",
		EnvRequestTimeoutMS:      "10000",
		EnvUnknownReleaseMinutes: "15",
	}
}

func mapLookup(values map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
