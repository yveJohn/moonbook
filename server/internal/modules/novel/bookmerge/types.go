package bookmerge

type TargetBookInput struct {
	BookName      string  `json:"bookName"`
	AuthorID      string  `json:"authorId"`
	CategoryCode  string  `json:"categoryCode"`
	WorkDirection *string `json:"workDirection"`
	Description   string  `json:"description"`
	BookStatus    string  `json:"bookStatus"`
	PublishStatus string  `json:"publishStatus"`
	OperatorName  string  `json:"operatorName"`
}

type PreviewInput struct {
	SourceBookIDs []string `json:"sourceBookIds"`
}

type ExecuteInput struct {
	SourceBookIDs      []string        `json:"sourceBookIds"`
	ExcludedChapterIDs []string        `json:"excludedChapterIds"`
	TargetBook         TargetBookInput `json:"targetBook"`
}

type EligibleBook struct {
	BookID          string `json:"bookId"`
	BookName        string `json:"bookName"`
	AuthorName      string `json:"authorName"`
	ChapterCount    string `json:"chapterCount"`
	ImportTaskID    string `json:"importTaskId"`
	SourceName      string `json:"sourceName"`
	ThreadTitle     string `json:"threadTitle"`
	ThreadCreatedAt string `json:"threadCreatedAt"`
	SortTimeSource  string `json:"sortTimeSource"`
}

type PreviewSource struct {
	SourceBookID       string `json:"sourceBookId"`
	SourceBookName     string `json:"sourceBookName"`
	SourceAuthorName   string `json:"sourceAuthorName"`
	SourceImportTaskID string `json:"sourceImportTaskId"`
	SourceName         string `json:"sourceName"`
	SourceThreadID     string `json:"sourceThreadId"`
	SourceThreadTitle  string `json:"sourceThreadTitle"`
	SourceThreadURL    string `json:"sourceThreadUrl"`
	SortTime           string `json:"sortTime"`
	SortTimeSource     string `json:"sortTimeSource"`
	SourceOrder        int    `json:"sourceOrder"`
	ChapterCount       string `json:"chapterCount"`
	OldPublishStatus   string `json:"oldPublishStatus"`
}

type PreviewChapter struct {
	SourceBookID      string `json:"sourceBookId"`
	SourceChapterID   string `json:"sourceChapterId"`
	SourceChapterNo   int    `json:"sourceChapterNo"`
	SourceChapterName string `json:"sourceChapterName"`
	TargetChapterNo   int    `json:"targetChapterNo"`
	TargetChapterName string `json:"targetChapterName"`
	ContentSource     string `json:"contentSource"`
	CleanResultID     string `json:"cleanResultId"`
	WordCount         string `json:"wordCount"`
	SortTime          string `json:"sortTime"`
	SortTimeSource    string `json:"sortTimeSource"`
	DuplicateFlag     bool   `json:"duplicateFlag"`
	DuplicateReason   string `json:"duplicateReason"`
	Excluded          bool   `json:"excluded"`
}

type Preview struct {
	Sources               []PreviewSource  `json:"sources"`
	Chapters              []PreviewChapter `json:"chapters"`
	ChapterCount          string           `json:"chapterCount"`
	DuplicateChapterCount string           `json:"duplicateChapterCount"`
	CleanContentCount     string           `json:"cleanContentCount"`
	OriginalContentCount  string           `json:"originalContentCount"`
}

type Task struct {
	ID                    string        `json:"id"`
	TargetBookID          string        `json:"targetBookId"`
	TargetBookName        string        `json:"targetBookName"`
	Status                string        `json:"status"`
	SourceCount           string        `json:"sourceCount"`
	ChapterCount          string        `json:"chapterCount"`
	IncludedChapterCount  string        `json:"includedChapterCount"`
	ExcludedChapterCount  string        `json:"excludedChapterCount"`
	DuplicateChapterCount string        `json:"duplicateChapterCount"`
	ContentPolicy         string        `json:"contentPolicy"`
	SortPolicy            string        `json:"sortPolicy"`
	SourceArchiveMode     string        `json:"sourceArchiveMode"`
	OperatorName          string        `json:"operatorName"`
	ErrorSummary          string        `json:"errorSummary"`
	StartTime             string        `json:"startTime"`
	EndTime               string        `json:"endTime"`
	CreatedAt             string        `json:"createdAt"`
	UpdatedAt             string        `json:"updatedAt"`
	Sources               []TaskSource  `json:"sources,omitempty"`
	Chapters              []TaskChapter `json:"chapters,omitempty"`
}

type TaskSource struct {
	ID                    string `json:"id"`
	SourceBookID          string `json:"sourceBookId"`
	SourceBookName        string `json:"sourceBookName"`
	SourceAuthorName      string `json:"sourceAuthorName"`
	SourceImportTaskID    string `json:"sourceImportTaskId"`
	SourceName            string `json:"sourceName"`
	SourceThreadID        string `json:"sourceThreadId"`
	SourceThreadTitle     string `json:"sourceThreadTitle"`
	SortTime              string `json:"sortTime"`
	SortTimeSource        string `json:"sortTimeSource"`
	SourceOrder           string `json:"sourceOrder"`
	OldPublishStatus      string `json:"oldPublishStatus"`
	ArchivedPublishStatus string `json:"archivedPublishStatus"`
	ArchiveTime           string `json:"archiveTime"`
}

type TaskChapter struct {
	ID                string `json:"id"`
	SourceBookID      string `json:"sourceBookId"`
	SourceChapterID   string `json:"sourceChapterId"`
	SourceChapterNo   string `json:"sourceChapterNo"`
	SourceChapterName string `json:"sourceChapterName"`
	TargetBookID      string `json:"targetBookId"`
	TargetChapterID   string `json:"targetChapterId"`
	TargetChapterNo   string `json:"targetChapterNo"`
	TargetChapterName string `json:"targetChapterName"`
	ContentSource     string `json:"contentSource"`
	DuplicateFlag     bool   `json:"duplicateFlag"`
	DuplicateReason   string `json:"duplicateReason"`
	Excluded          bool   `json:"excluded"`
	ExcludeReason     string `json:"excludeReason"`
}

type Page struct {
	Items    []Task `json:"list"`
	Total    int64  `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}
