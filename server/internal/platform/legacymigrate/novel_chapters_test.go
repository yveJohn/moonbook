package legacymigrate

import (
	"database/sql"
	"strings"
	"testing"
	"time"
)

func validLegacyChapter() legacyChapter {
	return legacyChapter{id: 1, bookID: 2, chapterNo: 0, chapterName: "第一章", wordCount: 4, isVIP: 0, bookPrice: 0, chapterStatus: "0", aiCleanStatus: "0", content: sql.NullString{String: "正文 test", Valid: true}, updatedAt: sql.NullTime{Time: time.Unix(1, 0), Valid: true}}
}

func TestValidateAndMapLegacyChapter(t *testing.T) {
	chapter := validLegacyChapter()
	if code, message := validateLegacyChapter(chapter); code != "" || message != "" {
		t.Fatalf("valid chapter code=%q message=%q", code, message)
	}
	if got := countLegacyChapterWords(chapter.content.String); got != 6 {
		t.Fatalf("word count=%d", got)
	}
	for raw, want := range map[string]string{"0": "enabled", "1": "disabled"} {
		if got, ok := mapLegacyChapterStatus(raw); !ok || got != want {
			t.Fatalf("chapter status %q=(%q,%v)", raw, got, ok)
		}
	}
	for raw, want := range map[string]string{"0": "pending", "2": "cleaned", "6": "skipped"} {
		if got, ok := mapLegacyCleanStatus(raw); !ok || got != want {
			t.Fatalf("clean status %q=(%q,%v)", raw, got, ok)
		}
	}
	invalid := validLegacyChapter()
	invalid.content = sql.NullString{String: strings.Repeat("x", legacyChapterMaxBytes+1), Valid: true}
	if code, _ := validateLegacyChapter(invalid); code != "INVALID_CHAPTER_CONTENT" {
		t.Fatalf("invalid content code=%q", code)
	}
}

func TestLegacyChapterFingerprintIsStableAndContentSensitive(t *testing.T) {
	chapter := validLegacyChapter()
	first := legacyChapterFingerprint(chapter, "正文")
	if first != legacyChapterFingerprint(chapter, "正文") || first == legacyChapterFingerprint(chapter, "正文变化") || len(first) != 64 {
		t.Fatalf("fingerprint behavior first=%q", first)
	}
}
