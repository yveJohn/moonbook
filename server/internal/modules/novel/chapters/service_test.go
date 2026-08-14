package chapters

import (
	"strings"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

func validInput() Input {
	content := " 第一章\n月色 good\t"
	chapterNo := 1
	return Input{BookID: 1, ChapterNo: &chapterNo, ChapterName: "第一章", IsVIP: true, BookPriceCoin: 12, ChapterStatus: "enabled", AICleanStatus: "pending", Content: &content}
}

func TestNormalizeInputAndCountWords(t *testing.T) {
	input, err := normalizeInput(validInput(), true)
	if err != nil {
		t.Fatal(err)
	}
	if countWords(*input.Content) != 9 {
		t.Fatalf("word count=%d", countWords(*input.Content))
	}
	invalidCases := []Input{
		func() Input { value := validInput(); value.BookID = 0; return value }(),
		func() Input { value := validInput(); value.ChapterStatus = "published"; return value }(),
		func() Input { value := validInput(); value.BookPriceCoin = -1; return value }(),
		func() Input {
			value := validInput()
			content := strings.Repeat("x", maxContentBytes+1)
			value.Content = &content
			return value
		}(),
	}
	for _, input := range invalidCases {
		if _, err := normalizeInput(input, true); apperror.Expose(err).Code != apperror.CodeInvalidArgument {
			t.Fatalf("normalizeInput(%+v) err=%v", input, err)
		}
	}
}

func TestCreateDefaultsMissingContentToEmpty(t *testing.T) {
	input := validInput()
	input.Content = nil
	normalized, err := normalizeInput(input, true)
	if err != nil || normalized.Content == nil || *normalized.Content != "" {
		t.Fatalf("normalized=%+v err=%v", normalized, err)
	}
}
