package legacymigrate

import "testing"

func TestShouldCheckLegacyProductTarget(t *testing.T) {
	for _, targetID := range []int64{-1, 0} {
		if shouldCheckLegacyProductTarget(targetID) {
			t.Fatalf("target ID %d should not participate in target uniqueness", targetID)
		}
	}
	if !shouldCheckLegacyProductTarget(1) {
		t.Fatal("positive target ID should participate in target uniqueness")
	}
}
