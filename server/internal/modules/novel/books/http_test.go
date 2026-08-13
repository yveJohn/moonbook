package books

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"math"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

func TestBookLongIDsSerializeAsStrings(t *testing.T) {
	max := int64(math.MaxInt64)
	data, err := json.Marshal(toResponse(Book{ID: max, PrimaryCategory: Category{ID: max}, AuthorID: max, LastChapterID: &max, FixedPriceCoin: &max, CreatedAt: time.Unix(0, 0), UpdatedAt: time.Unix(0, 0)}))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "authorId", "lastChapterId", "fixedPriceCoin"} {
		if decoded[key] != "9223372036854775807" {
			t.Fatalf("%s=%v JSON=%s", key, decoded[key], data)
		}
	}
	primary := decoded["primaryCategory"].(map[string]any)
	if primary["id"] != "9223372036854775807" {
		t.Fatalf("primary category=%v", primary)
	}
}

func TestListFilterRejectsInvalidPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, query := range []string{"?page=zero", "?page=0", "?pageSize=-1"} {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest("GET", "/novel/books"+query, nil)
		if _, err := listFilter(context); apperror.Expose(err).Code != apperror.CodeInvalidArgument {
			t.Fatalf("query=%q error=%v", query, err)
		}
	}
}
func TestParsePositiveLong(t *testing.T) {
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
}

func TestCoverTypeUsesFileSignature(t *testing.T) {
	tests := []struct {
		data      []byte
		wantType  string
		wantExt   string
		wantValid bool
	}{
		{data: []byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0x43}, wantType: "image/jpeg", wantExt: "jpg", wantValid: true},
		{data: []byte("\x89PNG\r\n\x1a\n"), wantType: "image/png", wantExt: "png", wantValid: true},
		{data: []byte("GIF89a"), wantType: "image/gif", wantExt: "gif", wantValid: true},
		{data: append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 20)...), wantType: "image/webp", wantExt: "webp", wantValid: true},
		{data: []byte("not an image"), wantValid: false},
	}
	for _, test := range tests {
		contentType, extension, valid := coverType(test.data)
		if contentType != test.wantType || extension != test.wantExt || valid != test.wantValid {
			t.Fatalf("coverType(%q)=(%q,%q,%v), want (%q,%q,%v)", test.data, contentType, extension, valid, test.wantType, test.wantExt, test.wantValid)
		}
	}
}
