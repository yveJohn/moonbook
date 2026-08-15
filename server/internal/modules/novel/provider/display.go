package provider

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	novelcontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/contract"
)

type Display struct{ db *sql.DB }

var _ novelcontract.DisplayReader = (*Display)(nil)

func NewDisplay(db *sql.DB) *Display { return &Display{db: db} }

func (provider *Display) BatchDisplay(ctx context.Context, request novelcontract.DisplayRequest) (novelcontract.DisplayBatch, error) {
	batch := novelcontract.DisplayBatch{
		Books:    make([]novelcontract.BookDisplay, len(request.BookIDs)),
		Chapters: make([]novelcontract.ChapterDisplay, len(request.ChapterIDs)),
	}
	if provider == nil || provider.db == nil {
		return batch, novelcontract.ErrUnavailable
	}
	books, err := provider.loadBooks(ctx, request.BookIDs)
	if err != nil {
		return batch, err
	}
	chapters, err := provider.loadChapters(ctx, request.ChapterIDs)
	if err != nil {
		return batch, err
	}
	for index, id := range request.BookIDs {
		batch.Books[index] = books[id]
		batch.Books[index].ID = id
	}
	for index, id := range request.ChapterIDs {
		batch.Chapters[index] = chapters[id]
		batch.Chapters[index].ID = id
	}
	return batch, nil
}

func (provider *Display) loadBooks(ctx context.Context, ids []int64) (map[int64]novelcontract.BookDisplay, error) {
	result := make(map[int64]novelcontract.BookDisplay, len(ids))
	query, args := displayQuery(`SELECT id,book_name,author_name,description,category_code,category_name,word_count,like_count,publish_status,deleted_at FROM novel_books WHERE id IN (%s)`, ids)
	if query == "" {
		return result, nil
	}
	rows, err := provider.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	defer rows.Close()
	for rows.Next() {
		var item novelcontract.BookDisplay
		var status string
		var deleted sql.NullTime
		if err := rows.Scan(&item.ID, &item.Name, &item.Author, &item.Description, &item.CategoryCode, &item.CategoryName, &item.WordCount, &item.LikeCount, &status, &deleted); err != nil {
			return nil, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
		}
		item.Found = true
		item.Published = status == "published" && !deleted.Valid
		result[item.ID] = item
	}
	if err := rows.Err(); err != nil {
		return nil, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	return result, nil
}

func (provider *Display) loadChapters(ctx context.Context, ids []int64) (map[int64]novelcontract.ChapterDisplay, error) {
	result := make(map[int64]novelcontract.ChapterDisplay, len(ids))
	query, args := displayQuery(`SELECT id,book_id,chapter_no,chapter_name,chapter_status,deleted_at FROM novel_chapters WHERE id IN (%s)`, ids)
	if query == "" {
		return result, nil
	}
	rows, err := provider.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	defer rows.Close()
	for rows.Next() {
		var item novelcontract.ChapterDisplay
		var status string
		var deleted sql.NullTime
		if err := rows.Scan(&item.ID, &item.BookID, &item.Number, &item.Name, &status, &deleted); err != nil {
			return nil, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
		}
		item.Found = true
		item.Enabled = status == "enabled" && !deleted.Valid
		result[item.ID] = item
	}
	if err := rows.Err(); err != nil {
		return nil, novelcontract.Wrap(novelcontract.ErrUnavailable, err)
	}
	return result, nil
}

func displayQuery(template string, ids []int64) (string, []any) {
	unique := make(map[int64]struct{}, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := unique[id]; exists {
			continue
		}
		unique[id] = struct{}{}
		args = append(args, id)
	}
	if len(args) == 0 {
		return "", nil
	}
	placeholders := make([]string, len(args))
	for index := range args {
		placeholders[index] = fmt.Sprintf("$%d", index+1)
	}
	return fmt.Sprintf(template, strings.Join(placeholders, ",")), args
}
