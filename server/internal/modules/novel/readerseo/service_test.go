package readerseo

import (
	"strings"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

func validInput() Input {
	return Input{
		SEOEnabled: true, IndexingEnabled: true, SitemapEnabled: true,
		SiteName: "月白书城", SiteURL: "https://ybsc.me/",
		DefaultDescription: "精选小说", HomeTitle: "月白书城", HomeDescription: "发现小说",
		BooksTitleTemplate: "{keyword} - {siteName}", BooksDescriptionTemplate: "{categoryName}{subCategoryName}",
		BookTitleTemplate:       "{bookName} - {authorName} - {siteName}",
		BookDescriptionTemplate: "{bookName}{categoryName}{bookDesc}",
	}
}

func TestNormalize(t *testing.T) {
	input := validInput()
	input.SiteName = "  月白书城  "
	clean, err := Normalize(input)
	if err != nil {
		t.Fatal(err)
	}
	if clean.SiteName != "月白书城" || clean.SiteURL != "https://ybsc.me" {
		t.Fatalf("normalized=%+v", clean)
	}
}

func TestNormalizeRejectsInvalidSiteURL(t *testing.T) {
	for _, value := range []string{"ftp://ybsc.me", "https://user@ybsc.me", "https://ybsc.me/path", "https://ybsc.me?q=1", "https://ybsc.me/#x", "https:///"} {
		input := validInput()
		input.SiteURL = value
		if _, err := Normalize(input); apperror.Expose(err).Code != apperror.CodeInvalidArgument {
			t.Fatalf("url=%q err=%v", value, err)
		}
	}
}

func TestNormalizeRejectsInvalidTemplates(t *testing.T) {
	for _, value := range []string{"{bookName}", "{unknown}", "{siteName", "siteName}", "{}"} {
		input := validInput()
		input.BooksTitleTemplate = value
		if _, err := Normalize(input); apperror.Expose(err).Code != apperror.CodeInvalidArgument {
			t.Fatalf("template=%q err=%v", value, err)
		}
	}
}

func TestNormalizeRejectsBlankAndOversizedFields(t *testing.T) {
	for _, mutate := range []func(*Input){
		func(input *Input) { input.SiteName = "  " },
		func(input *Input) { input.HomeTitle = strings.Repeat("长", 501) },
	} {
		input := validInput()
		mutate(&input)
		if _, err := Normalize(input); apperror.Expose(err).Code != apperror.CodeInvalidArgument {
			t.Fatalf("input=%+v err=%v", input, err)
		}
	}
}
