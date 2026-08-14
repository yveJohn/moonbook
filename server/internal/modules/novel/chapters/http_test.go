package chapters

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

func TestChapterLongValuesSerializeAsStrings(t *testing.T) {
	max := int64(math.MaxInt64)
	data, err := json.Marshal(toResponse(Chapter{ID: max, BookID: max, BookPriceCoin: max, CreatedAt: time.Unix(0, 0), UpdatedAt: time.Unix(0, 0)}))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "bookId", "bookPriceCoin"} {
		if decoded[key] != "9223372036854775807" {
			t.Fatalf("%s=%v JSON=%s", key, decoded[key], data)
		}
	}
}

func TestContentResponseFlattensChapterFields(t *testing.T) {
	data, err := json.Marshal(contentResponse{response: toResponse(Chapter{ID: 1, BookID: 2, CreatedAt: time.Unix(0, 0), UpdatedAt: time.Unix(0, 0)}), Content: "正文", Version: 3})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["id"] != "1" || decoded["bookId"] != "2" || decoded["content"] != "正文" || decoded["version"] != float64(3) {
		t.Fatalf("content response=%s", data)
	}
}

func TestParseChapterLongValues(t *testing.T) {
	for _, raw := range []string{"9223372036854775807", "9007199254740993"} {
		if _, err := parsePositiveLong(raw, "ID"); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{"9223372036854775808", "1.0", "+1", " 1", "0", ""} {
		if _, err := parsePositiveLong(raw, "ID"); apperror.Expose(err).Code != apperror.CodeInvalidArgument {
			t.Fatalf("raw=%q err=%v", raw, err)
		}
	}
	if value, err := parseNonNegativeLong("0", "价格"); err != nil || value != 0 {
		t.Fatalf("zero price value=%d err=%v", value, err)
	}
}

func TestParsePageRejectsInvalidValues(t *testing.T) {
	for _, raw := range []string{"zero", "0", "-1"} {
		if _, err := parsePage(raw, 1); apperror.Expose(err).Code != apperror.CodeInvalidArgument {
			t.Fatalf("raw=%q err=%v", raw, err)
		}
	}
}
