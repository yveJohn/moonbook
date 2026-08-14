package public

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

func TestSummaryKeepsLongIDsAsStrings(t *testing.T) {
	id := int64(math.MaxInt64)
	b := summary(Book{ID: id, LastChapterID: &id})
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["bookId"] != "9223372036854775807" || got["lastChapterId"] != "9223372036854775807" {
		t.Fatalf("unexpected IDs: %s", data)
	}
}

func TestAccessErrorUsesReaderCompatibilityCode(t *testing.T) {
	err := accessError("chapter_purchase_required")
	if got := apperror.Expose(err); got.HTTPStatus != 46104 || got.Message != "请先购买章节" {
		t.Fatalf("unexpected error: %+v", got)
	}
}
