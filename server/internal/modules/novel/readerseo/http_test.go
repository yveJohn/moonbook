package readerseo

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestConfigIDSerializesAsString(t *testing.T) {
	data, err := json.Marshal(toResponse(Config{ID: math.MaxInt64, CreatedAt: time.Unix(0, 0), UpdatedAt: time.Unix(0, 0)}))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["id"] != "9223372036854775807" {
		t.Fatalf("id=%v JSON=%s", decoded["id"], data)
	}
}
