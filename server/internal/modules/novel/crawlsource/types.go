package crawlsource

import "context"

type Source struct {
	ID, SourceName, BaseURL, RequestCharset, UserAgent, RequestIntervalMs, SortOrder, Remark, CreatedAt, UpdatedAt string
	CookieConfigured                                                                                               bool
	Enabled                                                                                                        bool
}

type Input struct {
	SourceName        string `json:"sourceName"`
	BaseURL           string `json:"baseUrl"`
	RequestCharset    string `json:"requestCharset"`
	CookieText        string `json:"cookieText"`
	UserAgent         string `json:"userAgent"`
	RequestIntervalMs string `json:"requestIntervalMs"`
	Enabled           bool   `json:"enabled"`
	SortOrder         string `json:"sortOrder"`
	Remark            string `json:"remark"`
}

type Repository interface {
	List(context.Context, string, string, int, int) ([]Source, int64, error)
	Create(context.Context, Input) (Source, error)
	Update(context.Context, int64, Input) (Source, error)
	Delete(context.Context, int64) error
}
