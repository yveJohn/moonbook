package public

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/catalog"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/gin-gonic/gin"
)

func TestSummaryKeepsLongIDsAsStrings(t *testing.T) {
	id := int64(math.MaxInt64)
	updatedAt := time.Date(2026, 8, 15, 6, 7, 8, 0, time.UTC)
	b := summary(Book{ID: id, LastChapterID: &id, LastChapterUpdatedAt: &updatedAt})
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
	if got["lastChapterUpdateTime"] != "2026-08-15 06:07:08" {
		t.Fatalf("unexpected date: %s", data)
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

func TestAccessErrorsUseReaderCompatibilityHTTPContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		reason, message string
		code            int
	}{
		{reason: "login_required", code: 46101, message: "请先登录后阅读"},
		{reason: "membership_required", code: 46102, message: "仅限会员阅读"},
		{reason: "book_purchase_required", code: 46103, message: "请先购买作品"},
		{reason: "chapter_purchase_required", code: 46104, message: "请先购买章节"},
		{reason: "unsupported_mode", code: 46105, message: "作品收费模式不可用"},
	}
	for _, test := range tests {
		t.Run(test.reason, func(t *testing.T) {
			err := accessError(test.reason)
			if got := apperror.Expose(err); got.HTTPStatus != test.code || got.Message != test.message {
				t.Fatalf("unexpected exposed error: %+v", got)
			}
			router := gin.New()
			router.GET("/access", func(c *gin.Context) { fail(c, err) })
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/access", nil))
			var body map[string]any
			if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if resp.Code != http.StatusOK || body["code"] != float64(test.code) || body["msg"] != test.message {
				t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
			}
		})
	}
}
