package legacymigrate

import "testing"

func TestNovelBookConversionRules(t *testing.T) {
	for _, test := range []struct{ old, want string }{{"0", "serializing"}, {"1", "completed"}} {
		if got, ok := mapLegacyBookStatus(test.old); !ok || got != test.want {
			t.Fatalf("mapLegacyBookStatus(%q)=(%q,%v)", test.old, got, ok)
		}
	}
	for _, test := range []struct{ old, want string }{{"0", "draft"}, {"1", "published"}, {"2", "deprecated"}} {
		if got, ok := mapLegacyPublishStatus(test.old); !ok || got != test.want {
			t.Fatalf("mapLegacyPublishStatus(%q)=(%q,%v)", test.old, got, ok)
		}
	}
	if _, ok := mapLegacyBookStatus("2"); ok {
		t.Fatal("unsupported book status was accepted")
	}
	if _, ok := mapLegacyPublishStatus("3"); ok {
		t.Fatal("unsupported publish status was accepted")
	}
	for _, value := range []string{"manual", "legacy", "txt_import", "forum_crawl"} {
		if !validLegacySourceType(value) {
			t.Fatalf("source type %q was rejected", value)
		}
	}
	for _, value := range []string{"word_charge", "membership_only", "login_free", "fixed_price"} {
		if !validLegacyChargeMode(value) {
			t.Fatalf("charge mode %q was rejected", value)
		}
	}
}

func TestNovelBookStageNames(t *testing.T) {
	if got := (NovelBooksStage{}).Name(); got != "novel-books" {
		t.Fatal(got)
	}
	if got := (NovelBookSubCategoriesStage{}).Name(); got != "novel-book-sub-categories" {
		t.Fatal(got)
	}
	if got := (NovelBookCoversStage{}).Name(); got != "novel-book-covers" {
		t.Fatal(got)
	}
}
