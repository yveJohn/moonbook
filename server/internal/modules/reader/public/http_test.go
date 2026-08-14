package public

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/catalog"
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

func TestProductStatusKeepsLongIDsAndLoginFreeContract(t *testing.T) {
	status := productStatus(normalizeBookStatus(catalog.AccessResult{
		BookID: "9223372036854775807", ChargeMode: "login_free", AccessReason: "free_chapter",
	}))
	if status["bookId"] != "9223372036854775807" || status["accessReason"] != "login_free" || status["readable"] != true {
		t.Fatalf("unexpected product status: %+v", status)
	}
	for _, key := range []string{"productId", "productName", "priceCoin", "saleStatus", "product"} {
		if status[key] != nil {
			t.Fatalf("%s should stay null without a fixed book product: %+v", key, status)
		}
	}
}

func TestAccessErrorUsesReaderCompatibilityCode(t *testing.T) {
	err := accessError("chapter_purchase_required")
	if got := apperror.Expose(err); got.HTTPStatus != 46104 || got.Message != "请先购买章节" {
		t.Fatalf("unexpected error: %+v", got)
	}
}
