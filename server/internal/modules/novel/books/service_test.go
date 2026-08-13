package books

import (
	"strings"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

func validInput() Input {
	price := int64(20)
	return Input{CategoryCode: "fantasy", BookName: "月下长书", AuthorID: 1, Score: "8.50",
		BookStatus: "serializing", PublishStatus: "draft", SourceType: "manual",
		ChargeMode: "fixed_price", FixedPriceCoin: &price, SubCategoryCodes: []string{"system", "system"}, Tags: []string{"群像"}}
}

func TestNormalizeInput(t *testing.T) {
	input, err := normalizeInput(validInput())
	if err != nil {
		t.Fatal(err)
	}
	if len(input.SubCategoryCodes) != 1 || input.Score != "8.50" || input.FixedPriceCoin == nil {
		t.Fatalf("normalized input = %+v", input)
	}
	invalidCases := []Input{
		func() Input { value := validInput(); value.Score = "10.01"; return value }(),
		func() Input {
			value := validInput()
			value.ChargeMode = "login_free"
			value.FixedPriceCoin = nil
			value.Tags = []string{strings.Repeat("长", 65)}
			return value
		}(),
		func() Input {
			value := validInput()
			value.ChargeMode = "fixed_price"
			value.FixedPriceCoin = nil
			return value
		}(),
	}
	for _, input := range invalidCases {
		if _, err := normalizeInput(input); apperror.Expose(err).Code != apperror.CodeInvalidArgument {
			t.Fatalf("normalizeInput(%+v) error=%v", input, err)
		}
	}
}
