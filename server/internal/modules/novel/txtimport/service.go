package txtimport

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
)

type Service struct{ Repo Repository }

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

func (s *Service) Retry(ctx context.Context, id int64) (Task, error) {
	if id <= 0 {
		return Task{}, errors.New("invalid TXT import task id")
	}
	return s.Repo.Retry(ctx, id)
}

func (s *Service) Cancel(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("invalid TXT import task id")
	}
	return s.Repo.Cancel(ctx, id)
}
