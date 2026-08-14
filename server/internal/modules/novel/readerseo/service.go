package readerseo

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

var placeholderPattern = regexp.MustCompile(`\{([A-Za-z][A-Za-z0-9]*)\}`)

var booksPlaceholders = map[string]struct{}{
	"siteName": {}, "keyword": {}, "categoryName": {}, "subCategoryName": {},
}

var bookPlaceholders = map[string]struct{}{
	"siteName": {}, "bookName": {}, "authorName": {}, "categoryName": {}, "bookDesc": {},
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func invalid(message string) error {
	return apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, message)
}

func Normalize(input Input) (Input, error) {
	input.SiteName = strings.TrimSpace(input.SiteName)
	input.SiteURL = strings.TrimSpace(input.SiteURL)
	input.DefaultDescription = strings.TrimSpace(input.DefaultDescription)
	input.HomeTitle = strings.TrimSpace(input.HomeTitle)
	input.HomeDescription = strings.TrimSpace(input.HomeDescription)
	input.BooksTitleTemplate = strings.TrimSpace(input.BooksTitleTemplate)
	input.BooksDescriptionTemplate = strings.TrimSpace(input.BooksDescriptionTemplate)
	input.BookTitleTemplate = strings.TrimSpace(input.BookTitleTemplate)
	input.BookDescriptionTemplate = strings.TrimSpace(input.BookDescriptionTemplate)

	fields := []struct {
		name  string
		value string
		max   int
	}{
		{"站点名称", input.SiteName, 100},
		{"站点域名", input.SiteURL, 2048},
		{"默认描述", input.DefaultDescription, 2000},
		{"首页标题", input.HomeTitle, 500},
		{"首页描述", input.HomeDescription, 2000},
		{"书库标题模板", input.BooksTitleTemplate, 500},
		{"书库描述模板", input.BooksDescriptionTemplate, 2000},
		{"书籍标题模板", input.BookTitleTemplate, 500},
		{"书籍描述模板", input.BookDescriptionTemplate, 2000},
	}
	for _, field := range fields {
		if field.value == "" || len([]rune(field.value)) > field.max {
			return Input{}, invalid(field.name + "不能为空且不能超过长度限制")
		}
	}

	normalizedURL, err := normalizeSiteURL(input.SiteURL)
	if err != nil {
		return Input{}, err
	}
	input.SiteURL = normalizedURL

	for _, template := range []struct {
		name    string
		value   string
		allowed map[string]struct{}
	}{
		{"书库标题模板", input.BooksTitleTemplate, booksPlaceholders},
		{"书库描述模板", input.BooksDescriptionTemplate, booksPlaceholders},
		{"书籍标题模板", input.BookTitleTemplate, bookPlaceholders},
		{"书籍描述模板", input.BookDescriptionTemplate, bookPlaceholders},
	} {
		if err := validateTemplate(template.value, template.allowed); err != nil {
			return Input{}, invalid(template.name + err.Error())
		}
	}
	return input, nil
}

func normalizeSiteURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Path != "" && parsed.Path != "/") {
		return "", invalid("站点域名必须是不含路径、认证、查询或片段的 HTTP/HTTPS 根地址")
	}
	parsed.Path = ""
	return parsed.String(), nil
}

func validateTemplate(value string, allowed map[string]struct{}) error {
	remaining := placeholderPattern.ReplaceAllStringFunc(value, func(match string) string { return "" })
	if strings.ContainsAny(remaining, "{}") {
		return errors.New("包含格式错误的占位符")
	}
	for _, match := range placeholderPattern.FindAllStringSubmatch(value, -1) {
		if _, ok := allowed[match[1]]; !ok {
			return errors.New("包含不支持的占位符：" + match[1])
		}
	}
	return nil
}

const selectConfig = `SELECT id, seo_enabled, indexing_enabled, sitemap_enabled, site_name, site_url,
	default_description, home_title, home_description, books_title_template,
	books_description_template, book_title_template, book_description_template, created_at, updated_at
	FROM novel_reader_seo_config WHERE id=1`

func scanConfig(scanner interface{ Scan(...any) error }) (Config, error) {
	var config Config
	err := scanner.Scan(&config.ID, &config.SEOEnabled, &config.IndexingEnabled, &config.SitemapEnabled,
		&config.SiteName, &config.SiteURL, &config.DefaultDescription, &config.HomeTitle,
		&config.HomeDescription, &config.BooksTitleTemplate, &config.BooksDescriptionTemplate,
		&config.BookTitleTemplate, &config.BookDescriptionTemplate, &config.CreatedAt, &config.UpdatedAt)
	return config, err
}

func (service *Service) Get(ctx context.Context) (Config, error) {
	config, err := scanConfig(service.db.QueryRowContext(ctx, selectConfig))
	if errors.Is(err, sql.ErrNoRows) {
		return Config{}, apperror.New(apperror.CodeInternal, http.StatusInternalServerError, "SEO配置不存在")
	}
	if err != nil {
		return Config{}, apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "查询SEO配置失败")
	}
	return config, nil
}

func (service *Service) Update(ctx context.Context, input Input) (Config, error) {
	clean, err := Normalize(input)
	if err != nil {
		return Config{}, err
	}
	tx, err := service.db.BeginTx(ctx, nil)
	if err != nil {
		return Config{}, apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "保存SEO配置失败")
	}
	defer tx.Rollback()
	if err := tx.QueryRowContext(ctx, `SELECT id FROM novel_reader_seo_config WHERE id=1 FOR UPDATE`).Scan(new(int64)); errors.Is(err, sql.ErrNoRows) {
		return Config{}, apperror.New(apperror.CodeInternal, http.StatusInternalServerError, "SEO配置不存在")
	} else if err != nil {
		return Config{}, apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "保存SEO配置失败")
	}
	row := tx.QueryRowContext(ctx, `UPDATE novel_reader_seo_config SET seo_enabled=$1, indexing_enabled=$2,
		sitemap_enabled=$3, site_name=$4, site_url=$5, default_description=$6, home_title=$7,
		home_description=$8, books_title_template=$9, books_description_template=$10,
		book_title_template=$11, book_description_template=$12, updated_at=now() WHERE id=1
		RETURNING id, seo_enabled, indexing_enabled, sitemap_enabled, site_name, site_url,
		default_description, home_title, home_description, books_title_template,
		books_description_template, book_title_template, book_description_template, created_at, updated_at`,
		clean.SEOEnabled, clean.IndexingEnabled, clean.SitemapEnabled, clean.SiteName, clean.SiteURL,
		clean.DefaultDescription, clean.HomeTitle, clean.HomeDescription, clean.BooksTitleTemplate,
		clean.BooksDescriptionTemplate, clean.BookTitleTemplate, clean.BookDescriptionTemplate)
	config, err := scanConfig(row)
	if err != nil {
		return Config{}, apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "保存SEO配置失败")
	}
	if err := tx.Commit(); err != nil {
		return Config{}, apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "保存SEO配置失败")
	}
	return config, nil
}
