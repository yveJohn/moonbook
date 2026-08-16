package epusdt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
)

var lowercaseSignaturePattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func Canonical(fields map[string]string) string {
	keys := make([]string, 0, len(fields))
	for key, value := range fields {
		if key != "signature" && value != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+fields[key])
	}
	return strings.Join(parts, "&")
}

func Sign(fields map[string]string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(Canonical(fields)))
	return hex.EncodeToString(mac.Sum(nil))
}

func Verify(fields map[string]string, signature, secret string) bool {
	if !lowercaseSignaturePattern.MatchString(signature) {
		return false
	}
	return hmac.Equal([]byte(Sign(fields, secret)), []byte(signature))
}
