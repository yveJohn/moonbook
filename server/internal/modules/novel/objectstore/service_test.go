package objectstore

import (
	"errors"
	"testing"
)

func TestNormalizeTargetAndObjectKey(t *testing.T) {
	chapter, err := normalizeTarget(Target{Kind: KindChapterContent, BookID: 42, OwnerID: 42, Extension: "html"})
	if err != nil {
		t.Fatal(err)
	}
	if chapter.Extension != "txt" || objectKey(chapter, 7) != "chapters/42/42/v7.txt" {
		t.Fatalf("chapter target = %+v key=%s", chapter, objectKey(chapter, 7))
	}
	clean, err := normalizeTarget(Target{Kind: KindChapterClean, BookID: 42, OwnerID: 99})
	if err != nil || clean.Extension != "txt" || objectKey(clean, 2) != "chapter-clean/42/99/v2.txt" {
		t.Fatalf("clean target = %+v key=%s err=%v", clean, objectKey(clean, 2), err)
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

func TestDetectCover(t *testing.T) {
	for _, test := range []struct {
		data        []byte
		contentType string
		extension   string
	}{
		{[]byte{0xff, 0xd8, 0xff, 0xdb, 0, 0}, "image/jpeg", "jpg"},
		{[]byte("\x89PNG\r\n\x1a\n"), "image/png", "png"},
		{[]byte("GIF89a"), "image/gif", "gif"},
		{append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 20)...), "image/webp", "webp"},
	} {
		contentType, extension, ok := DetectCover(test.data)
		if !ok || contentType != test.contentType || extension != test.extension {
			t.Fatalf("DetectCover()=(%q,%q,%v)", contentType, extension, ok)
		}
	}
	if _, _, ok := DetectCover([]byte("not an image")); ok {
		t.Fatal("plain text should not be a cover")
	}
	if !errors.Is(ErrActiveObjectNotFound, ErrActiveObjectNotFound) {
		t.Fatal("not-found sentinel must support errors.Is")
	}
}
