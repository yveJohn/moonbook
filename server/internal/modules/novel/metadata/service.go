package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/jackc/pgx/v5/pgconn"
)

const maxPageSize = 100

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func normalizeFilter(filter ListFilter) ListFilter {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 10
	}
	if filter.PageSize > maxPageSize {
		filter.PageSize = maxPageSize
	}
	filter.Keyword = strings.TrimSpace(filter.Keyword)
	filter.Kind = strings.TrimSpace(filter.Kind)
	filter.Status = strings.TrimSpace(filter.Status)
	return filter
}

func normalizeCategory(input CategoryInput) (CategoryInput, error) {
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	input.Kind = strings.TrimSpace(input.Kind)
	if input.WorkDirection != nil {
		value := strings.TrimSpace(*input.WorkDirection)
		if value == "" {
			input.WorkDirection = nil
		} else {
			input.WorkDirection = &value
		}
	}
	if input.Code == "" || len(input.Code) > 64 || input.Name == "" || len([]rune(input.Name)) > 100 {
		return CategoryInput{}, invalid("分类编码和名称不能为空，且不能超过字段长度限制")
	}
	if input.Kind != CategoryKindPrimary && input.Kind != CategoryKindSub {
		return CategoryInput{}, invalid("分类类型必须是 primary 或 sub")
	}
	if input.Sort < 0 {
		return CategoryInput{}, invalid("排序不能小于 0")
	}
	return input, nil
}

func normalizeAuthor(input AuthorInput) (AuthorInput, string, error) {
	input.PenName = strings.TrimSpace(input.PenName)
	input.Status = strings.TrimSpace(input.Status)
	if input.WorkDirection != nil {
		value := strings.TrimSpace(*input.WorkDirection)
		if value == "" {
			input.WorkDirection = nil
		} else {
			input.WorkDirection = &value
		}
	}
	if input.PenName == "" || len([]rune(input.PenName)) > 100 {
		return AuthorInput{}, "", invalid("作者名称不能为空，且不能超过 100 个字符")
	}
	if input.Status != AuthorStatusPending && input.Status != AuthorStatusActive && input.Status != AuthorStatusBlocked {
		return AuthorInput{}, "", invalid("作者状态无效")
	}
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, input.PenName)
	return input, normalized, nil
}

func invalid(message string) error {
	return apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, message)
}

func notFound(entity string) error {
	return apperror.New(apperror.CodeNotFound, http.StatusNotFound, entity+"不存在")
}

func mapWriteError(err error, entity string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return apperror.Wrap(err, apperror.CodeConflict, http.StatusConflict, entity+"已存在")
	}
	return apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "保存"+entity+"失败")
}

func (service *Service) ListCategories(ctx context.Context, raw ListFilter) (Page[Category], error) {
	filter := normalizeFilter(raw)
	args := []any{"%" + filter.Keyword + "%", filter.Kind}
	where := `deleted_at IS NULL AND ($1 = '%%' OR code ILIKE $1 OR name ILIKE $1) AND ($2 = '' OR kind = $2)`
	if filter.Enabled != nil {
		args = append(args, *filter.Enabled)
		where += fmt.Sprintf(" AND enabled = $%d", len(args))
	}
	var total int64
	if err := service.db.QueryRowContext(ctx, "SELECT count(*) FROM novel_categories WHERE "+where, args...).Scan(&total); err != nil {
		return Page[Category]{}, apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "查询分类失败")
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	query := fmt.Sprintf(`SELECT id, code, name, kind, work_direction, sort, enabled, source, created_at, updated_at
		FROM novel_categories WHERE %s ORDER BY kind, sort, id LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args))
	rows, err := service.db.QueryContext(ctx, query, args...)
	if err != nil {
		return Page[Category]{}, apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "查询分类失败")
	}
	defer rows.Close()
	items := make([]Category, 0)
	for rows.Next() {
		var item Category
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Kind, &item.WorkDirection, &item.Sort, &item.Enabled, &item.Source, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return Page[Category]{}, apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "读取分类失败")
		}
		items = append(items, item)
	}
	return Page[Category]{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, rows.Err()
}

func (service *Service) CreateCategory(ctx context.Context, input CategoryInput) (Category, error) {
	clean, err := normalizeCategory(input)
	if err != nil {
		return Category{}, err
	}
	var item Category
	err = service.db.QueryRowContext(ctx, `INSERT INTO novel_categories
		(code, name, kind, work_direction, sort, enabled) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, code, name, kind, work_direction, sort, enabled, source, created_at, updated_at`,
		clean.Code, clean.Name, clean.Kind, clean.WorkDirection, clean.Sort, clean.Enabled).
		Scan(&item.ID, &item.Code, &item.Name, &item.Kind, &item.WorkDirection, &item.Sort, &item.Enabled, &item.Source, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Category{}, mapWriteError(err, "分类")
	}
	return item, nil
}

func (service *Service) UpdateCategory(ctx context.Context, id int64, input CategoryInput) (Category, error) {
	clean, err := normalizeCategory(input)
	if err != nil {
		return Category{}, err
	}
	var item Category
	err = service.db.QueryRowContext(ctx, `UPDATE novel_categories SET code=$2, name=$3, kind=$4,
		work_direction=$5, sort=$6, enabled=$7, updated_at=now() WHERE id=$1 AND deleted_at IS NULL
		RETURNING id, code, name, kind, work_direction, sort, enabled, source, created_at, updated_at`,
		id, clean.Code, clean.Name, clean.Kind, clean.WorkDirection, clean.Sort, clean.Enabled).
		Scan(&item.ID, &item.Code, &item.Name, &item.Kind, &item.WorkDirection, &item.Sort, &item.Enabled, &item.Source, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Category{}, notFound("分类")
	}
	if err != nil {
		return Category{}, mapWriteError(err, "分类")
	}
	return item, nil
}

func (service *Service) DeleteCategory(ctx context.Context, id int64) error {
	result, err := service.db.ExecContext(ctx, `UPDATE novel_categories SET deleted_at=now(), enabled=false, updated_at=now()
		WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "删除分类失败")
	}
	count, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "删除分类失败")
	}
	if count == 0 {
		return notFound("分类")
	}
	return nil
}

func (service *Service) ListAuthors(ctx context.Context, raw ListFilter) (Page[Author], error) {
	filter := normalizeFilter(raw)
	args := []any{"%" + filter.Keyword + "%", filter.Status, filter.PageSize, (filter.Page - 1) * filter.PageSize}
	where := `deleted_at IS NULL AND ($1 = '%%' OR pen_name ILIKE $1) AND ($2 = '' OR status = $2)`
	var total int64
	if err := service.db.QueryRowContext(ctx, "SELECT count(*) FROM novel_authors WHERE "+where, args[:2]...).Scan(&total); err != nil {
		return Page[Author]{}, apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "查询作者失败")
	}
	rows, err := service.db.QueryContext(ctx, `SELECT id, pen_name, normalized_name, status, work_direction, source,
		legacy_author_id, legacy_book_author_id, created_at, updated_at FROM novel_authors WHERE `+where+` ORDER BY id DESC LIMIT $3 OFFSET $4`, args...)
	if err != nil {
		return Page[Author]{}, apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "查询作者失败")
	}
	defer rows.Close()
	items := make([]Author, 0)
	for rows.Next() {
		var item Author
		if err := rows.Scan(&item.ID, &item.PenName, &item.NormalizedName, &item.Status, &item.WorkDirection, &item.Source, &item.LegacyAuthorID, &item.LegacyBookAuthorID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return Page[Author]{}, apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "读取作者失败")
		}
		items = append(items, item)
	}
	return Page[Author]{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, rows.Err()
}

func (service *Service) CreateAuthor(ctx context.Context, input AuthorInput) (Author, error) {
	clean, normalized, err := normalizeAuthor(input)
	if err != nil {
		return Author{}, err
	}
	var item Author
	err = service.db.QueryRowContext(ctx, `INSERT INTO novel_authors (pen_name, normalized_name, status, work_direction)
		VALUES ($1, $2, $3, $4) RETURNING id, pen_name, normalized_name, status, work_direction, source,
		legacy_author_id, legacy_book_author_id, created_at, updated_at`, clean.PenName, normalized, clean.Status, clean.WorkDirection).
		Scan(&item.ID, &item.PenName, &item.NormalizedName, &item.Status, &item.WorkDirection, &item.Source, &item.LegacyAuthorID, &item.LegacyBookAuthorID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Author{}, mapWriteError(err, "作者")
	}
	return item, nil
}

func (service *Service) UpdateAuthor(ctx context.Context, id int64, input AuthorInput) (Author, error) {
	clean, normalized, err := normalizeAuthor(input)
	if err != nil {
		return Author{}, err
	}
	var item Author
	err = service.db.QueryRowContext(ctx, `UPDATE novel_authors SET pen_name=$2, normalized_name=$3, status=$4,
		work_direction=$5, updated_at=now() WHERE id=$1 AND deleted_at IS NULL RETURNING id, pen_name,
		normalized_name, status, work_direction, source, legacy_author_id, legacy_book_author_id, created_at, updated_at`,
		id, clean.PenName, normalized, clean.Status, clean.WorkDirection).
		Scan(&item.ID, &item.PenName, &item.NormalizedName, &item.Status, &item.WorkDirection, &item.Source, &item.LegacyAuthorID, &item.LegacyBookAuthorID, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Author{}, notFound("作者")
	}
	if err != nil {
		return Author{}, mapWriteError(err, "作者")
	}
	return item, nil
}

func (service *Service) DeleteAuthor(ctx context.Context, id int64) error {
	result, err := service.db.ExecContext(ctx, `UPDATE novel_authors SET deleted_at=now(), status='blocked', updated_at=now()
		WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "删除作者失败")
	}
	count, err := result.RowsAffected()
	if err != nil {
		return apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "删除作者失败")
	}
	if count == 0 {
		return notFound("作者")
	}
	return nil
}
