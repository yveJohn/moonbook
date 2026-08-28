package secretcrypto

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func testKey(fill byte) string {
	return base64.StdEncoding.EncodeToString(bytesOf(fill, 32))
}

func bytesOf(value byte, count int) []byte {
	out := make([]byte, count)
	for i := range out {
		out[i] = value
	}
	return out
}

func TestCipherRoundTripUsesRandomNonce(t *testing.T) {
	c, err := New(testKey(7))
	if err != nil {
		t.Fatal(err)
	}
	scope := Scope{Table: "reader_payment_channels", RecordID: "1", Field: "secret"}
	first, err := c.Encrypt("sensitive-value", scope)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Encrypt("sensitive-value", scope)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(first, "v1:") {
		t.Fatalf("cipher envelopes are not versioned and randomized")
	}
	plaintext, err := c.Decrypt(first, scope)
	if err != nil || plaintext != "sensitive-value" {
		t.Fatalf("round trip failed: plaintext=%q err=%v", plaintext, err)
	}
}

func TestCipherRejectsTamperingAndScopeReuse(t *testing.T) {
	c, err := New(testKey(9))
	if err != nil {
		t.Fatal(err)
	}
	scope := Scope{Table: "reader_payment_channels", RecordID: "1", Field: "secret"}
	envelope, err := c.Encrypt("do-not-leak", scope)
	if err != nil {
		t.Fatal(err)
	}
	tampered := envelope[:len(envelope)-1] + "A"
	for _, test := range []struct {
		name  string
		value string
		scope Scope
	}{
		{name: "tampered", value: tampered, scope: scope},
		{name: "other record", value: envelope, scope: Scope{Table: scope.Table, RecordID: "2", Field: scope.Field}},
		{name: "other field", value: envelope, scope: Scope{Table: scope.Table, RecordID: scope.RecordID, Field: "pid"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if value, err := c.Decrypt(test.value, test.scope); !errors.Is(err, ErrInvalidCiphertext) || strings.Contains(err.Error(), "do-not-leak") || value != "" {
				t.Fatalf("value=%q err=%v", value, err)
			}
		})
	}
}

func TestMasterKeyAndScopeValidation(t *testing.T) {
	for _, value := range []string{"", "not-base64", base64.StdEncoding.EncodeToString(bytesOf(1, 31))} {
		if _, err := New(value); !errors.Is(err, ErrInvalidMasterKey) {
			t.Fatalf("key %q err=%v", value, err)
		}
	}
	c, err := NewFromEnv(func(key string) (string, bool) {
		if key != EnvMasterKey {
			t.Fatalf("unexpected key %q", key)
		}
		return testKey(3), true
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Encrypt("value", Scope{}); !errors.Is(err, ErrInvalidScope) {
		t.Fatalf("scope err=%v", err)
	}
}
