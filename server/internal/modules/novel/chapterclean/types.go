package chapterclean

import "time"

type Config struct {
	ID                        string  `json:"id"`
	AIConfigID                string  `json:"aiConfigId"`
	RefusalFallbackAIConfigID string  `json:"refusalFallbackAiConfigId"`
	Enabled                   bool    `json:"enabled"`
	SystemPrompt              string  `json:"systemPrompt"`
	Temperature               float64 `json:"temperature"`
	MaxTokens                 int     `json:"maxTokens"`
	MinChapterWordCount       int     `json:"minChapterWordCount"`
	MinCleanedTextPercent     int     `json:"minCleanedTextPercent"`
	AutoSuccessWordCount      int     `json:"autoSuccessWordCount"`
	TimeoutSeconds            int     `json:"timeoutSeconds"`
	RequestIntervalMS         int     `json:"requestIntervalMs"`
	RetryCount                int     `json:"retryCount"`
	ContinueOnFailure         bool    `json:"continueOnFailure"`
	CreatedAt                 string  `json:"createdAt"`
	UpdatedAt                 string  `json:"updatedAt"`
}
type ConfigInput struct {
	AIConfigID                string  `json:"aiConfigId"`
	RefusalFallbackAIConfigID string  `json:"refusalFallbackAiConfigId"`
	Enabled                   bool    `json:"enabled"`
	SystemPrompt              string  `json:"systemPrompt"`
	Temperature               float64 `json:"temperature"`
	MaxTokens                 int     `json:"maxTokens"`
	MinChapterWordCount       int     `json:"minChapterWordCount"`
	MinCleanedTextPercent     int     `json:"minCleanedTextPercent"`
	AutoSuccessWordCount      int     `json:"autoSuccessWordCount"`
	TimeoutSeconds            int     `json:"timeoutSeconds"`
	RequestIntervalMS         int     `json:"requestIntervalMs"`
	RetryCount                int     `json:"retryCount"`
	ContinueOnFailure         bool    `json:"continueOnFailure"`
}
type Task struct {
	ID                   string     `json:"id"`
	BookID               string     `json:"bookId"`
	BookName             string     `json:"bookName"`
	Status               string     `json:"status"`
	ForceReclean         bool       `json:"forceReclean"`
	TotalCount           int        `json:"totalCount"`
	ProcessedCount       int        `json:"processedCount"`
	SuccessCount         int        `json:"successCount"`
	DiscardCount         int        `json:"discardCount"`
	FailCount            int        `json:"failCount"`
	SkipCount            int        `json:"skipCount"`
	AllChaptersDiscarded bool       `json:"allChaptersDiscarded"`
	StopRequested        bool       `json:"stopRequested"`
	OperatorName         string     `json:"operatorName"`
	ErrorSummary         string     `json:"errorSummary"`
	StartedAt            time.Time  `json:"startedAt"`
	FinishedAt           *time.Time `json:"finishedAt"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}
type Result struct {
	ID                 string    `json:"id"`
	TaskID             string    `json:"taskId"`
	BookID             string    `json:"bookId"`
	ChapterID          string    `json:"chapterId"`
	BookName           string    `json:"bookName"`
	ChapterName        string    `json:"chapterName"`
	SourceChapterNo    int       `json:"sourceChapterNo"`
	ContentType        string    `json:"contentType"`
	IsNovelBody        *bool     `json:"isNovelBody"`
	CleanedChapterName string    `json:"cleanedChapterName"`
	CleanedText        string    `json:"cleanedText,omitempty"`
	OriginalText       string    `json:"originalText,omitempty"`
	ChapterSummary     string    `json:"chapterSummary"`
	CleanedWordCount   int       `json:"cleanedWordCount"`
	RemovedNonNovel    *bool     `json:"removedNonNovel"`
	Confidence         *float64  `json:"confidence"`
	RawResponse        string    `json:"rawResponse"`
	Status             string    `json:"status"`
	ErrorMessage       string    `json:"errorMessage"`
	Active             bool      `json:"active"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}
type StartInput struct {
	BookID       string `json:"bookId"`
	ForceReclean bool   `json:"forceReclean"`
	OperatorName string `json:"operatorName"`
}
type ReviewInput struct {
	Status             string `json:"status"`
	CleanedChapterName string `json:"cleanedChapterName"`
	CleanedText        string `json:"cleanedText"`
}
