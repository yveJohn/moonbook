package metadata

import (
	"encoding/json"
	"math"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/gin-gonic/gin"
)

func TestLongIDsSerializeAsStrings(t *testing.T) {
	maxID := int64(math.MaxInt64)
	categoryJSON, err := json.Marshal(toCategoryResponse(Category{ID: maxID, CreatedAt: time.Unix(0, 0), UpdatedAt: time.Unix(0, 0)}))
	if err != nil {
		t.Fatal(err)
	}
	if string(categoryJSON) == "" || !containsJSONID(categoryJSON, `"id":"9223372036854775807"`) {
		t.Fatalf("category JSON does not preserve bigint as string: %s", categoryJSON)
	}
	authorJSON, err := json.Marshal(toAuthorResponse(Author{ID: maxID, LegacyAuthorID: &maxID, LegacyBookAuthorID: &maxID, CreatedAt: time.Unix(0, 0), UpdatedAt: time.Unix(0, 0)}))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"id":"9223372036854775807"`, `"legacyAuthorId":"9223372036854775807"`, `"legacyBookAuthorId":"9223372036854775807"`} {
		if !containsJSONID(authorJSON, want) {
			t.Fatalf("author JSON missing %s: %s", want, authorJSON)
		}
	}
}

func containsJSONID(data []byte, value string) bool {
	return len(data) >= len(value) && string(data) != "" && indexOf(string(data), value) >= 0
}

func indexOf(value, target string) int {
	for index := 0; index+len(target) <= len(value); index++ {
		if value[index:index+len(target)] == target {
			return index
		}
	}
	return -1
}

func TestPathIDAcceptsMaxInt64AndRejectsUnsafeForms(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		value string
		want  int64
		ok    bool
	}{
		{value: "9223372036854775807", want: math.MaxInt64, ok: true},
		{value: "9007199254740993", want: 9007199254740993, ok: true},
		{value: "9223372036854775808", ok: false},
		{value: "1.0", ok: false},
		{value: "+1", ok: false},
		{value: " 1", ok: false},
		{value: "0", ok: false},
	} {
		t.Run(test.value, func(t *testing.T) {
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			context.Params = gin.Params{{Key: "id", Value: test.value}}
			got, err := pathID(context)
			if test.ok && (err != nil || got != test.want) {
				t.Fatalf("pathID() = %d, %v; want %d", got, err, test.want)
			}
			if !test.ok && apperror.Expose(err).Code != apperror.CodeInvalidArgument {
				t.Fatalf("pathID() error = %v, want invalid argument", err)
			}
		})
	}
}

func TestInputNormalization(t *testing.T) {
	category, err := normalizeCategory(CategoryInput{Code: " fantasy ", Name: " 玄幻 ", Kind: CategoryKindPrimary, Sort: 2, Enabled: true})
	if err != nil || category.Code != "fantasy" || category.Name != "玄幻" {
		t.Fatalf("normalizeCategory() = %#v, %v", category, err)
	}
	author, normalized, err := normalizeAuthor(AuthorInput{PenName: " 青 山 ", Status: AuthorStatusActive})
	if err != nil || author.PenName != "青 山" || normalized != "青山" {
		t.Fatalf("normalizeAuthor() = %#v, %q, %v", author, normalized, err)
	}
}
