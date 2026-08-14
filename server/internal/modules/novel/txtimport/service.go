package txtimport

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/importtask"
)

type Service struct{ Repo Repository }

const (
	maxPreviewChapters = 20
	maxPreviewRunes    = 2000
)

type PreviewChapter struct {
	Title     string `json:"title"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

type Preview struct {
	TaskID            string           `json:"taskId"`
	OriginalFilename  string           `json:"originalFilename"`
	TotalChapterCount int              `json:"totalChapterCount"`
	Chapters          []PreviewChapter `json:"chapters"`
}

func NewService(repo Repository) *Service { return &Service{Repo: repo} }

func validBookID(raw string) error {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return errors.New("invalid target book id")
	}
	return nil
}

func validateFile(name string, data []byte) error {
	name = strings.TrimSpace(name)
	if name == "" || filepath.Base(name) != name || len([]rune(name)) > 255 || !strings.EqualFold(filepath.Ext(name), ".txt") {
		return errors.New("only TXT files up to 255 characters are supported")
	}
	if len(data) == 0 || int64(len(data)) > MaxFileBytes {
		return errors.New("TXT file must be between 1 byte and 16 MiB")
	}
	return nil
}

func (s *Service) Upload(ctx context.Context, store FileStore, bookID, filename, operator string, data []byte) (Task, error) {
	bookID = strings.TrimSpace(bookID)
	filename = strings.TrimSpace(filename)
	operator = strings.TrimSpace(operator)
	if err := validBookID(bookID); err != nil {
		return Task{}, err
	}
	if err := validateFile(filename, data); err != nil {
		return Task{}, err
	}
	if len([]rune(operator)) > 64 {
		return Task{}, errors.New("operator name is too long")
	}
	hash := sha256Hex(data)
	meta, err := store.Put(ctx, "imports/txt/"+hash+".txt", data, "text/plain; charset=utf-8")
	if err != nil {
		return Task{}, err
	}
	return s.Repo.Create(ctx, CreateInput{TargetBookID: bookID, OriginalFilename: filename, OperatorName: operator, Object: meta})
}

func (s *Service) List(ctx context.Context, keyword, status string, page, size int) ([]Task, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return s.Repo.List(ctx, strings.TrimSpace(keyword), strings.TrimSpace(status), page, size)
}

func (s *Service) Get(ctx context.Context, id int64) (Task, error) {
	if id <= 0 {
		return Task{}, errors.New("invalid TXT import task id")
	}
	return s.Repo.Get(ctx, id)
}

func (s *Service) Preview(ctx context.Context, store FileStore, id int64) (Preview, error) {
	if id <= 0 {
		return Preview{}, errors.New("invalid TXT import task id")
	}
	if store == nil {
		return Preview{}, errors.New("TXT file store is not configured")
	}
	task, err := s.Repo.Get(ctx, id)
	if err != nil {
		return Preview{}, err
	}
	size, err := strconv.ParseInt(task.ObjectByteSize, 10, 64)
	if err != nil || size <= 0 {
		return Preview{}, errors.New("invalid TXT object size")
	}
	data, err := store.Get(ctx, ObjectMeta{Key: task.ObjectKey, SHA256: task.ObjectSHA256, ByteSize: size})
	if err != nil {
		return Preview{}, err
	}
	parsed := importtask.ParseTXTChapters(data, strings.TrimSuffix(task.OriginalFilename, filepath.Ext(task.OriginalFilename)))
	preview := Preview{TaskID: task.ID, OriginalFilename: task.OriginalFilename, TotalChapterCount: len(parsed), Chapters: make([]PreviewChapter, 0, min(len(parsed), maxPreviewChapters))}
	for _, chapter := range parsed {
		if len(preview.Chapters) >= maxPreviewChapters {
			break
		}
		content := []rune(chapter.Content)
		truncated := len(content) > maxPreviewRunes
		if truncated {
			content = content[:maxPreviewRunes]
		}
		preview.Chapters = append(preview.Chapters, PreviewChapter{Title: chapter.Title, Content: string(content), Truncated: truncated})
	}
	return preview, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *Service) Retry(ctx context.Context, id int64) (Task, error) {
	if id <= 0 {
		return Task{}, errors.New("invalid TXT import task id")
	}
	return s.Repo.Retry(ctx, id)
}

func (s *Service) ReplaceFile(ctx context.Context, store FileStore, id int64, filename string, data []byte) (Task, error) {
	if id <= 0 {
		return Task{}, errors.New("invalid TXT import task id")
	}
	filename = strings.TrimSpace(filename)
	if err := validateFile(filename, data); err != nil {
		return Task{}, err
	}
	if store == nil {
		return Task{}, errors.New("TXT file store is not configured")
	}
	hash := sha256Hex(data)
	meta, err := store.Put(ctx, "imports/txt/"+hash+".txt", data, "text/plain; charset=utf-8")
	if err != nil {
		return Task{}, err
	}
	return s.Repo.ReplaceFile(ctx, id, filename, meta)
}

func (s *Service) Cancel(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("invalid TXT import task id")
	}
	return s.Repo.Cancel(ctx, id)
}
