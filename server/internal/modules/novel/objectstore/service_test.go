package objectstore

import "testing"

func TestNormalizeTargetAndObjectKey(t *testing.T) {
	chapter, err := normalizeTarget(Target{Kind: KindChapterContent, BookID: 42, OwnerID: 42, Extension: "html"})
	if err != nil {
		t.Fatal(err)
	}
	if chapter.Extension != "txt" || objectKey(chapter, 7) != "chapters/42/42/v7.txt" {
		t.Fatalf("chapter target = %+v key=%s", chapter, objectKey(chapter, 7))
	}
	cover, err := normalizeTarget(Target{Kind: KindBookCover, BookID: 42, OwnerID: 42, Extension: ".WEBP"})
	if err != nil {
		t.Fatal(err)
	}
	if cover.Extension != "webp" || objectKey(cover, 3) != "covers/42/v3.webp" {
		t.Fatalf("cover target = %+v key=%s", cover, objectKey(cover, 3))
	}
	for _, target := range []Target{
		{Kind: KindChapterContent, BookID: 0, OwnerID: 1},
		{Kind: KindBookCover, BookID: 1, OwnerID: 2, Extension: "png"},
		{Kind: KindBookCover, BookID: 1, OwnerID: 1, Extension: "../png"},
		{Kind: "attachment", BookID: 1, OwnerID: 1},
	} {
		if _, err := normalizeTarget(target); err == nil {
			t.Fatalf("normalizeTarget(%+v) should fail", target)
		}
	}
}

func TestSafeExtension(t *testing.T) {
	if got := SafeExtension("cover.Final.WEBP"); got != "webp" {
		t.Fatalf("SafeExtension() = %q", got)
	}
}
