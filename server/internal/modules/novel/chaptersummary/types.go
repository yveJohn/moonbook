package chaptersummary

import "time"

type Config struct {
	ID                        string  `json:"id"`
	Enabled                   bool    `json:"enabled"`
	AIConfigID                string  `json:"aiConfigId"`
	RefusalFallbackAIConfigID string  `json:"refusalFallbackAiConfigId"`
	SystemPrompt              string  `json:"systemPrompt"`
	Temperature               float64 `json:"temperature"`
	MaxTokens                 int     `json:"maxTokens"`
	MaxInputChars             int     `json:"maxInputChars"`
	TimeoutSeconds            int     `json:"timeoutSeconds"`
	RequestIntervalMS         int     `json:"requestIntervalMs"`
	RetryCount                int     `json:"retryCount"`
	BatchSize                 int     `json:"batchSize"`
	CreatedAt                 string  `json:"createdAt"`
	UpdatedAt                 string  `json:"updatedAt"`
}

type ConfigInput struct {
	Enabled                   bool    `json:"enabled"`
	AIConfigID                string  `json:"aiConfigId"`
	RefusalFallbackAIConfigID string  `json:"refusalFallbackAiConfigId"`
	SystemPrompt              string  `json:"systemPrompt"`
	Temperature               float64 `json:"temperature"`
	MaxTokens                 int     `json:"maxTokens"`
	MaxInputChars             int     `json:"maxInputChars"`
	TimeoutSeconds            int     `json:"timeoutSeconds"`
	RequestIntervalMS         int     `json:"requestIntervalMs"`
	RetryCount                int     `json:"retryCount"`
	BatchSize                 int     `json:"batchSize"`
}

type Task struct {
	ID                   string     `json:"id"`
	Status               string     `json:"status"`
	Automatic            bool       `json:"automatic"`
	TotalCount           int        `json:"totalCount"`
	ProcessedCount       int        `json:"processedCount"`
	SuccessCount         int        `json:"successCount"`
	FailCount            int        `json:"failCount"`
	CurrentResultID      string     `json:"currentResultId"`
	CurrentBookID        string     `json:"currentBookId"`
	CurrentChapterID     string     `json:"currentChapterId"`
	StopRequested        bool       `json:"stopRequested"`
	OperatorName         string     `json:"operatorName"`
	ErrorSummary         string     `json:"errorSummary"`
	LatestErrorMessage   string     `json:"latestErrorMessage"`
	LatestRawResponse    string     `json:"latestRawResponse,omitempty"`
	RawResponseExpiresAt *time.Time `json:"rawResponseExpiresAt"`
	StartedAt            time.Time  `json:"startedAt"`
	FinishedAt           *time.Time `json:"finishedAt"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

type Status struct {
	Config          Config `json:"config"`
	RunningTask     *Task  `json:"runningTask"`
	LatestTask      *Task  `json:"latestTask"`
	PendingCount    int64  `json:"pendingCount"`
	ProgressPercent int    `json:"progressPercent"`
	Enabled         bool   `json:"enabled"`
	Running         bool   `json:"running"`
}

type StartInput struct {
	OperatorName string `json:"operatorName"`
}
