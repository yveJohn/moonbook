package epusdt

import (
	"fmt"
	"strings"
	"testing"
)

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
