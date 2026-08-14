package bookprofile

import "time"

type Config struct {
	ID                        string  `json:"id"`
	AutoScanEnabled           bool    `json:"autoScanEnabled"`
	AutoApplyEnabled          bool    `json:"autoApplyEnabled"`
	ProcessAIRefusalEnabled   bool    `json:"processAiRefusalEnabled"`
	AIConfigID                string  `json:"aiConfigId"`
	RefusalFallbackAIConfigID string  `json:"refusalFallbackAiConfigId"`
	SystemPrompt              string  `json:"systemPrompt"`
	Temperature               float64 `json:"temperature"`
	MaxTokens                 int     `json:"maxTokens"`
	TimeoutSeconds            int     `json:"timeoutSeconds"`
	RequestIntervalMS         int     `json:"requestIntervalMs"`
	RetryCount                int     `json:"retryCount"`
	ScanBatchSize             int     `json:"scanBatchSize"`
	MaxInputChars             int     `json:"maxInputChars"`
}

type SnapshotCategory struct {
	Code string `json:"categoryCode"`
	Name string `json:"categoryName"`
	Sort int    `json:"sort"`
}
type BookSnapshot struct {
	BookName      string             `json:"bookName"`
	CategoryCode  string             `json:"categoryCode"`
	CategoryName  string             `json:"categoryName"`
	BookDesc      string             `json:"bookDesc"`
	SubCategories []SnapshotCategory `json:"subCategories"`
}
type SuggestedSnapshot struct {
	BookName              string             `json:"bookName"`
	CategoryCode          string             `json:"categoryCode"`
	CategoryName          string             `json:"categoryName"`
	UnmatchedCategoryName string             `json:"unmatchedCategoryName"`
	CategoryReason        string             `json:"categoryReason"`
	SubCategories         []SnapshotCategory `json:"subCategories"`
	SubCategoryReason     string             `json:"subCategoryReason"`
	BookDesc              string             `json:"bookDesc"`
	Confidence            float64            `json:"confidence"`
	Reason                string             `json:"reason"`
}

type Suggestion struct {
	ID            string             `json:"id"`
	BookID        string             `json:"bookId"`
	BookName      string             `json:"bookName"`
	Status        string             `json:"status"`
	TriggerType   string             `json:"triggerType"`
	InputMode     string             `json:"inputMode"`
	InputDigest   string             `json:"inputDigest"`
	InputSnapshot any                `json:"inputSnapshot,omitempty"`
	Original      BookSnapshot       `json:"original"`
	Suggested     *SuggestedSnapshot `json:"suggested,omitempty"`
	Review        *BookSnapshot      `json:"review,omitempty"`
	RawResponse   string             `json:"rawResponse,omitempty"`
	ReviewerName  string             `json:"reviewerName"`
	RejectReason  string             `json:"rejectReason"`
	ReviewedAt    *time.Time         `json:"reviewedAt"`
	AppliedAt     *time.Time         `json:"appliedAt"`
	ErrorMessage  string             `json:"errorMessage"`
	FailureType   string             `json:"failureType"`
	RetrySourceID string             `json:"retrySourceId"`
	RetryTargetID string             `json:"retryTargetId"`
	CreatedAt     time.Time          `json:"createdAt"`
	UpdatedAt     time.Time          `json:"updatedAt"`
}
type Filter struct {
	BookID                                               string
	BookName, Status, TriggerType, SuggestedCategoryCode string
	Page, PageSize                                       int
}
type GenerateInput struct {
	BookID             string `json:"bookId"`
	Regenerate         bool   `json:"regenerate"`
	SourceSuggestionID string `json:"sourceSuggestionId"`
}
type BatchInput struct {
	SuggestionIDs []string `json:"suggestionIds"`
}
type BatchFailure struct {
	SuggestionID string `json:"suggestionId"`
	Message      string `json:"message"`
}
type BatchResult struct {
	TotalCount   int            `json:"totalCount"`
	SuccessCount int            `json:"successCount"`
	FailureCount int            `json:"failureCount"`
	Failures     []BatchFailure `json:"failures"`
}
type ReviewInput struct {
	BookName         string   `json:"bookName"`
	CategoryCode     string   `json:"categoryCode"`
	BookDesc         string   `json:"bookDesc"`
	SubCategoryCodes []string `json:"subCategoryCodes"`
	RejectReason     string   `json:"rejectReason"`
	ReviewerName     string   `json:"reviewerName"`
}
