package readerseo

import "time"

type Config struct {
	ID                       int64
	SEOEnabled               bool
	IndexingEnabled          bool
	SitemapEnabled           bool
	SiteName                 string
	SiteURL                  string
	DefaultDescription       string
	HomeTitle                string
	HomeDescription          string
	BooksTitleTemplate       string
	BooksDescriptionTemplate string
	BookTitleTemplate        string
	BookDescriptionTemplate  string
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type Input struct {
	SEOEnabled               bool
	IndexingEnabled          bool
	SitemapEnabled           bool
	SiteName                 string
	SiteURL                  string
	DefaultDescription       string
	HomeTitle                string
	HomeDescription          string
	BooksTitleTemplate       string
	BooksDescriptionTemplate string
	BookTitleTemplate        string
	BookDescriptionTemplate  string
}
