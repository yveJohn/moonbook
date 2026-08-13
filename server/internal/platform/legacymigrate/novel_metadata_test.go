package legacymigrate

import "testing"

func TestNovelMetadataConversionRules(t *testing.T) {
	if got := normalizeLegacyName(" 青 山 \t"); got != "青山" {
		t.Fatalf("normalizeLegacyName() = %q", got)
	}
	for _, test := range []struct {
		old  int
		want string
	}{
		{old: 0, want: "pending"},
		{old: 1, want: "active"},
		{old: 2, want: "blocked"},
		{old: 99, want: "blocked"},
	} {
		if got := mapBookAuthorStatus(test.old); got != test.want {
			t.Fatalf("mapBookAuthorStatus(%d) = %q, want %q", test.old, got, test.want)
		}
	}
}

func TestNovelMetadataStageNamesAndCursor(t *testing.T) {
	wants := map[Stage]string{
		NovelCategoryDictionaryStage{}:          "novel-category-dictionary",
		LegacyBookCategoryStage{}:               "legacy-book-category",
		NovelBookAuthorStage{}:                  "novel-book-author",
		LegacyAuthorTableStage{Table: "author"}: "legacy-author",
	}
	for stage, want := range wants {
		if got := stage.Name(); got != want {
			t.Fatalf("stage.Name() = %q, want %q", got, want)
		}
	}
	if value, err := parseCursor("9223372036854775807"); err != nil || value != 9223372036854775807 {
		t.Fatalf("parseCursor(max) = %d, %v", value, err)
	}
	for _, value := range []string{"-1", "1.5", "bad"} {
		if _, err := parseCursor(value); err == nil {
			t.Fatalf("parseCursor(%q) returned nil error", value)
		}
	}
}
