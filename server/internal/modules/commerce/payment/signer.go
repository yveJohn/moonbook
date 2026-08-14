package payment

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"sort"
	"strings"
)

func Canonical(fields map[string]string) string {
	keys := make([]string, 0, len(fields))
	for k, v := range fields {
		if k != "signature" && v != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+fields[k])
	}
	return strings.Join(parts, "&")
}
func Sign(fields map[string]string, secret string) string {
	m := hmac.New(sha256.New, []byte(secret))
	_, _ = m.Write([]byte(Canonical(fields)))
	return hex.EncodeToString(m.Sum(nil))
}
func Verify(fields map[string]string, signature, secret string) bool {
	if len(signature) != 64 {
		return false
	}
	actual, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(Sign(fields, secret))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
