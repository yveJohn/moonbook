package invite

import (
	"regexp"
	"testing"
)

func TestGeneratedCodeIsEightAlphanumeric(t *testing.T) {
	re := regexp.MustCompile(`^[0-9A-Z]{8}$`)
	seen := map[string]struct{}{}
	for i := 0; i < 32; i++ {
		code, err := generatedCode()
		if err != nil {
			t.Fatal(err)
		}
		if !re.MatchString(code) {
			t.Fatalf("code=%q", code)
		}
		seen[code] = struct{}{}
	}
	if len(seen) < 30 {
		t.Fatalf("too many collisions: %d unique", len(seen))
	}
}
