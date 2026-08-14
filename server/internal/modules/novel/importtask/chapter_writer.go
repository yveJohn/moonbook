package importtask

import (
	"context"
	"errors"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/chapters"
	"strconv"
)

type ChaptersWriter struct{ Service *chapters.Service }

func (w ChaptersWriter) Import(ctx context.Context, task Task, parsed []ParsedChapter) (int, error) {
	if w.Service == nil {
		return 0, errors.New("chapter service is required")
	}
	bookID, err := strconv.ParseInt(task.TargetBookID, 10, 64)
	if err != nil || bookID <= 0 {
		return 0, errors.New("target book is required for chapter import")
	}
	imported := 0
	sourceType := "forum_crawl"
	if task.ImportMode == "txt" {
		sourceType = "txt_import"
	}
	for i, item := range parsed {
		title := item.Title
		if title == "" {
			title = "未命名章节"
		}
		content := item.Content
		if content == "" {
			continue
		}
		no := i + 1
		exists, checkErr := w.Service.HasChapterNo(ctx, bookID, no)
		if checkErr != nil {
			return imported, checkErr
		}
		if exists {
			continue
		}
		if _, err = w.Service.Create(ctx, chapters.Input{BookID: bookID, ChapterNo: &no, ChapterName: title, ChapterStatus: "enabled", AICleanStatus: "pending", SourceType: sourceType, Content: &content}); err != nil {
			return imported, err
		}
		imported++
	}
	return imported, nil
}
