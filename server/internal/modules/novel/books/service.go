package books

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/jackc/pgx/v5/pgconn"
)

const maxPageSize = 100

var decimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]{1,2})?$`)

type Service struct{ db *sql.DB }

func NewService(db *sql.DB) *Service { return &Service{db: db} }

func invalid(message string) error {
	return apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, message)
}
func notFound() error {
	return apperror.New(apperror.CodeNotFound, http.StatusNotFound, "书籍不存在")
}

func normalizeInput(input Input) (Input, error) {
	input.CategoryCode = strings.TrimSpace(input.CategoryCode)
	input.LegacyCoverURL = strings.TrimSpace(input.LegacyCoverURL)
	input.BookName = strings.TrimSpace(input.BookName)
	input.Description = strings.TrimSpace(input.Description)
	input.Score = strings.TrimSpace(input.Score)
	input.BookStatus = strings.TrimSpace(input.BookStatus)
	input.PublishStatus = strings.TrimSpace(input.PublishStatus)
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.ChargeMode = strings.TrimSpace(input.ChargeMode)
	input.FeaturedNote = strings.TrimSpace(input.FeaturedNote)
	if input.WorkDirection != nil {
		value := strings.TrimSpace(*input.WorkDirection)
		if value == "" {
			input.WorkDirection = nil
		} else {
			input.WorkDirection = &value
		}
	}
	if input.BookName == "" || len([]rune(input.BookName)) > 100 || input.AuthorID <= 0 || input.CategoryCode == "" {
		return Input{}, invalid("书名、作者和主分类不能为空")
	}
	if len(input.LegacyCoverURL) > 500 || len([]rune(input.Description)) > 2000 || len([]rune(input.FeaturedNote)) > 255 {
		return Input{}, invalid("书籍文本超过字段长度限制")
	}
	if !decimalPattern.MatchString(input.Score) {
		return Input{}, invalid("评分必须是 0 到 10 之间且最多两位小数")
	}
	score, _ := strconv.ParseFloat(input.Score, 64)
	if score > 10 {
		return Input{}, invalid("评分必须是 0 到 10 之间且最多两位小数")
	}
	if input.BookStatus != "serializing" && input.BookStatus != "completed" {
		return Input{}, invalid("作品状态无效")
	}
	if input.PublishStatus != "draft" && input.PublishStatus != "published" && input.PublishStatus != "deprecated" {
		return Input{}, invalid("发布状态无效")
	}
	if input.SourceType != "manual" && input.SourceType != "legacy" && input.SourceType != "txt_import" && input.SourceType != "forum_crawl" {
		return Input{}, invalid("来源类型无效")
	}
	if input.ChargeMode != "word_charge" && input.ChargeMode != "membership_only" && input.ChargeMode != "login_free" && input.ChargeMode != "fixed_price" {
		return Input{}, invalid("收费模式无效")
	}
	if input.ChargeMode == "fixed_price" {
		if input.FixedPriceCoin == nil || *input.FixedPriceCoin <= 0 {
			return Input{}, invalid("整书售价必须大于 0")
		}
	} else {
		input.FixedPriceCoin = nil
	}
	if input.FeaturedSort < 0 {
		return Input{}, invalid("精选排序不能小于 0")
	}
	var err error
	input.SubCategoryCodes, err = uniqueTrimmed(input.SubCategoryCodes, 64)
	if err != nil {
		return Input{}, invalid("副分类编码不能为空且不能超过 64 个字符")
	}
	input.Tags, err = uniqueTrimmed(input.Tags, 64)
	if err != nil {
		return Input{}, invalid("标签不能为空且不能超过 64 个字符")
	}
	if len(input.SubCategoryCodes) > 20 || len(input.Tags) > 20 {
		return Input{}, invalid("副分类和标签最多各 20 个")
	}
	return input, nil
}

func uniqueTrimmed(values []string, maxRunes int) ([]string, error) {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" || len([]rune(value)) > maxRunes {
			return nil, errors.New("value is blank or too long")
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func mapWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return apperror.Wrap(err, apperror.CodeConflict, http.StatusConflict, "同名作者下已存在该作品")
		case "23503", "23514":
			return apperror.Wrap(err, apperror.CodeInvalidArgument, http.StatusBadRequest, "书籍关联或字段状态无效")
		}
	}
	return apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "保存书籍失败")
}

func (service *Service) resolveRelations(ctx context.Context, tx *sql.Tx, input Input) (Category, string, []Category, error) {
	var primary Category
	err := tx.QueryRowContext(ctx, `SELECT id,code,name FROM novel_categories WHERE kind='primary' AND code=$1 AND enabled AND deleted_at IS NULL`, input.CategoryCode).Scan(&primary.ID, &primary.Code, &primary.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return Category{}, "", nil, invalid("主分类不存在或已停用")
	}
	if err != nil {
		return Category{}, "", nil, err
	}
	var authorName string
	err = tx.QueryRowContext(ctx, `SELECT pen_name FROM novel_authors WHERE id=$1 AND status='active' AND deleted_at IS NULL`, input.AuthorID).Scan(&authorName)
	if errors.Is(err, sql.ErrNoRows) {
		return Category{}, "", nil, invalid("作者不存在或状态不可用")
	}
	if err != nil {
		return Category{}, "", nil, err
	}
	subs := make([]Category, 0, len(input.SubCategoryCodes))
	for _, code := range input.SubCategoryCodes {
		var category Category
		err := tx.QueryRowContext(ctx, `SELECT id,code,name FROM novel_categories WHERE kind='sub' AND code=$1 AND enabled AND deleted_at IS NULL`, code).Scan(&category.ID, &category.Code, &category.Name)
		if errors.Is(err, sql.ErrNoRows) {
			return Category{}, "", nil, invalid("副分类不存在或已停用")
		}
		if err != nil {
			return Category{}, "", nil, err
		}
		subs = append(subs, category)
	}
	return primary, authorName, subs, nil
}

func (service *Service) Create(ctx context.Context, raw Input) (Book, error) {
	input, err := normalizeInput(raw)
	if err != nil {
		return Book{}, err
	}
	tx, err := service.db.BeginTx(ctx, nil)
	if err != nil {
		return Book{}, err
	}
	defer tx.Rollback()
	primary, authorName, subs, err := service.resolveRelations(ctx, tx, input)
	if err != nil {
		return Book{}, err
	}
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO novel_books (work_direction,primary_category_id,category_code,category_name,legacy_cover_url,book_name,author_id,author_name,description,score,book_status,publish_status,source_type,featured,featured_sort,featured_note,charge_mode,fixed_price_coin)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18) RETURNING id`, input.WorkDirection, primary.ID, primary.Code, primary.Name, input.LegacyCoverURL, input.BookName, input.AuthorID, authorName, input.Description, input.Score, input.BookStatus, input.PublishStatus, input.SourceType, input.Featured, input.FeaturedSort, input.FeaturedNote, input.ChargeMode, input.FixedPriceCoin).Scan(&id)
	if err != nil {
		return Book{}, mapWriteError(err)
	}
	if err := replaceRelations(ctx, tx, id, subs, input.Tags); err != nil {
		return Book{}, err
	}
	if err := tx.Commit(); err != nil {
		return Book{}, err
	}
	return service.Get(ctx, id)
}

func (service *Service) Update(ctx context.Context, id int64, raw Input) (Book, error) {
	input, err := normalizeInput(raw)
	if err != nil {
		return Book{}, err
	}
	tx, err := service.db.BeginTx(ctx, nil)
	if err != nil {
		return Book{}, err
	}
	defer tx.Rollback()
	primary, authorName, subs, err := service.resolveRelations(ctx, tx, input)
	if err != nil {
		return Book{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE novel_books SET work_direction=$2,primary_category_id=$3,category_code=$4,category_name=$5,legacy_cover_url=$6,book_name=$7,author_id=$8,author_name=$9,description=$10,score=$11,book_status=$12,publish_status=$13,source_type=$14,featured=$15,featured_sort=$16,featured_note=$17,charge_mode=$18,fixed_price_coin=$19,updated_at=now() WHERE id=$1 AND deleted_at IS NULL`, id, input.WorkDirection, primary.ID, primary.Code, primary.Name, input.LegacyCoverURL, input.BookName, input.AuthorID, authorName, input.Description, input.Score, input.BookStatus, input.PublishStatus, input.SourceType, input.Featured, input.FeaturedSort, input.FeaturedNote, input.ChargeMode, input.FixedPriceCoin)
	if err != nil {
		return Book{}, mapWriteError(err)
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return Book{}, notFound()
	}
	if err := replaceRelations(ctx, tx, id, subs, input.Tags); err != nil {
		return Book{}, err
	}
	if err := tx.Commit(); err != nil {
		return Book{}, err
	}
	return service.Get(ctx, id)
}

func replaceRelations(ctx context.Context, tx *sql.Tx, bookID int64, subs []Category, tags []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM novel_book_sub_categories WHERE book_id=$1`, bookID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM novel_book_tags WHERE book_id=$1`, bookID); err != nil {
		return err
	}
	for sort, category := range subs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO novel_book_sub_categories (book_id,category_id,category_code,category_name,sort) VALUES ($1,$2,$3,$4,$5)`, bookID, category.ID, category.Code, category.Name, sort); err != nil {
			return mapWriteError(err)
		}
	}
	for sort, tag := range tags {
		if _, err := tx.ExecContext(ctx, `INSERT INTO novel_book_tags (book_id,tag,sort) VALUES ($1,$2,$3)`, bookID, tag, sort); err != nil {
			return mapWriteError(err)
		}
	}
	return nil
}

func (service *Service) Get(ctx context.Context, id int64) (Book, error) {
	book, err := service.scanOne(ctx, `WHERE b.id=$1 AND b.deleted_at IS NULL`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Book{}, notFound()
	}
	if err != nil {
		return Book{}, apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "查询书籍失败")
	}
	if err := service.loadRelations(ctx, &book); err != nil {
		return Book{}, err
	}
	return book, nil
}

func (service *Service) scanOne(ctx context.Context, where string, args ...any) (Book, error) {
	var b Book
	err := service.db.QueryRowContext(ctx, `SELECT b.id,b.work_direction,b.primary_category_id,b.category_code,b.category_name,b.legacy_cover_url,b.book_name,b.author_id,b.author_name,b.description,b.score::text,b.book_status,b.publish_status,b.source_type,b.featured,b.featured_sort,b.featured_note,b.visit_count,b.like_count,b.word_count,b.comment_count,b.yesterday_buy,b.last_chapter_id,b.last_chapter_name,b.last_chapter_updated_at,b.charge_mode,b.fixed_price_coin,b.created_at,b.updated_at FROM novel_books b `+where, args...).Scan(&b.ID, &b.WorkDirection, &b.PrimaryCategory.ID, &b.PrimaryCategory.Code, &b.PrimaryCategory.Name, &b.LegacyCoverURL, &b.BookName, &b.AuthorID, &b.AuthorName, &b.Description, &b.Score, &b.BookStatus, &b.PublishStatus, &b.SourceType, &b.Featured, &b.FeaturedSort, &b.FeaturedNote, &b.VisitCount, &b.LikeCount, &b.WordCount, &b.CommentCount, &b.YesterdayBuy, &b.LastChapterID, &b.LastChapterName, &b.LastChapterUpdatedAt, &b.ChargeMode, &b.FixedPriceCoin, &b.CreatedAt, &b.UpdatedAt)
	return b, err
}

func (service *Service) loadRelations(ctx context.Context, book *Book) error {
	books := []Book{*book}
	if err := service.loadPageRelations(ctx, books); err != nil {
		return err
	}
	*book = books[0]
	return nil
}

func (service *Service) loadPageRelations(ctx context.Context, books []Book) error {
	if len(books) == 0 {
		return nil
	}
	positions := make(map[int64]int, len(books))
	args := make([]any, 0, len(books))
	placeholders := make([]string, 0, len(books))
	for index := range books {
		books[index].SubCategories = []Category{}
		books[index].Tags = []string{}
		positions[books[index].ID] = index
		args = append(args, books[index].ID)
		placeholders = append(placeholders, "$"+strconv.Itoa(index+1))
	}
	ids := strings.Join(placeholders, ",")
	rows, err := service.db.QueryContext(ctx, `SELECT book_id,category_id,category_code,category_name FROM novel_book_sub_categories WHERE book_id IN (`+ids+`) ORDER BY book_id,sort,category_id`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var bookID int64
		var c Category
		if err := rows.Scan(&bookID, &c.ID, &c.Code, &c.Name); err != nil {
			return err
		}
		books[positions[bookID]].SubCategories = append(books[positions[bookID]].SubCategories, c)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	tagRows, err := service.db.QueryContext(ctx, `SELECT book_id,tag FROM novel_book_tags WHERE book_id IN (`+ids+`) ORDER BY book_id,sort,tag`, args...)
	if err != nil {
		return err
	}
	defer tagRows.Close()
	for tagRows.Next() {
		var bookID int64
		var tag string
		if err := tagRows.Scan(&bookID, &tag); err != nil {
			return err
		}
		books[positions[bookID]].Tags = append(books[positions[bookID]].Tags, tag)
	}
	return tagRows.Err()
}

func (service *Service) List(ctx context.Context, filter Filter) (Page, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 10
	}
	if filter.PageSize > maxPageSize {
		filter.PageSize = maxPageSize
	}
	args := []any{"%" + strings.TrimSpace(filter.Keyword) + "%", strings.TrimSpace(filter.CategoryCode), strings.TrimSpace(filter.BookStatus), strings.TrimSpace(filter.PublishStatus), strings.TrimSpace(filter.SourceType), strings.TrimSpace(filter.ChargeMode)}
	where := `b.deleted_at IS NULL AND ($1='%%' OR b.book_name ILIKE $1 OR b.author_name ILIKE $1) AND ($2='' OR b.category_code=$2) AND ($3='' OR b.book_status=$3) AND ($4='' OR b.publish_status=$4) AND ($5='' OR b.source_type=$5) AND ($6='' OR b.charge_mode=$6)`
	var total int64
	if err := service.db.QueryRowContext(ctx, "SELECT count(*) FROM novel_books b WHERE "+where, args...).Scan(&total); err != nil {
		return Page{}, err
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := service.db.QueryContext(ctx, `SELECT b.id,b.work_direction,b.primary_category_id,b.category_code,b.category_name,b.legacy_cover_url,b.book_name,b.author_id,b.author_name,b.description,b.score::text,b.book_status,b.publish_status,b.source_type,b.featured,b.featured_sort,b.featured_note,b.visit_count,b.like_count,b.word_count,b.comment_count,b.yesterday_buy,b.last_chapter_id,b.last_chapter_name,b.last_chapter_updated_at,b.charge_mode,b.fixed_price_coin,b.created_at,b.updated_at FROM novel_books b WHERE `+where+` ORDER BY b.updated_at DESC,b.id DESC LIMIT $7 OFFSET $8`, args...)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	items := []Book{}
	for rows.Next() {
		var b Book
		if err := rows.Scan(&b.ID, &b.WorkDirection, &b.PrimaryCategory.ID, &b.PrimaryCategory.Code, &b.PrimaryCategory.Name, &b.LegacyCoverURL, &b.BookName, &b.AuthorID, &b.AuthorName, &b.Description, &b.Score, &b.BookStatus, &b.PublishStatus, &b.SourceType, &b.Featured, &b.FeaturedSort, &b.FeaturedNote, &b.VisitCount, &b.LikeCount, &b.WordCount, &b.CommentCount, &b.YesterdayBuy, &b.LastChapterID, &b.LastChapterName, &b.LastChapterUpdatedAt, &b.ChargeMode, &b.FixedPriceCoin, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return Page{}, err
		}
		items = append(items, b)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	if err := service.loadPageRelations(ctx, items); err != nil {
		return Page{}, err
	}
	return Page{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func (service *Service) Delete(ctx context.Context, id int64) error {
	result, err := service.db.ExecContext(ctx, `UPDATE novel_books SET deleted_at=now(),publish_status='deprecated',featured=false,updated_at=now() WHERE id=$1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		return notFound()
	}
	return nil
}
