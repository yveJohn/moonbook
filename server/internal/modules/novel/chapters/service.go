package chapters

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	maxPageSize     = 100
	maxContentBytes = (16 << 20) - 1
)

type Service struct {
	db      *sql.DB
	objects *objectstore.Service
}

func NewService(db *sql.DB, objects *objectstore.Service) *Service {
	return &Service{db: db, objects: objects}
}

func invalid(message string) error {
	return apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, message)
}

func chapterNotFound() error {
	return apperror.New(apperror.CodeNotFound, http.StatusNotFound, "章节不存在")
}

func normalizeInput(input Input, creating bool) (Input, error) {
	input.ChapterName = strings.TrimSpace(input.ChapterName)
	input.ChapterStatus = strings.TrimSpace(input.ChapterStatus)
	input.AICleanStatus = strings.TrimSpace(input.AICleanStatus)
	if input.BookID <= 0 || input.ChapterName == "" || len([]rune(input.ChapterName)) > 255 {
		return Input{}, invalid("书籍和章节名称不能为空，章节名称不能超过 255 个字符")
	}
	if input.ChapterNo != nil && *input.ChapterNo < 0 {
		return Input{}, invalid("章节序号不能小于 0")
	}
	if input.BookPriceCoin < 0 {
		return Input{}, invalid("章节价格不能小于 0")
	}
	if input.ChapterStatus != "enabled" && input.ChapterStatus != "disabled" {
		return Input{}, invalid("章节状态无效")
	}
	switch input.AICleanStatus {
	case "pending", "cleaning", "cleaned", "discarded", "failed", "expired", "skipped":
	default:
		return Input{}, invalid("AI 清洗状态无效")
	}
	if creating && input.Content == nil {
		empty := ""
		input.Content = &empty
	}
	if input.Content != nil {
		if !utf8.ValidString(*input.Content) || len(*input.Content) > maxContentBytes {
			return Input{}, invalid("章节正文必须是 UTF-8 且不能超过 16 MiB")
		}
	}
	return input, nil
}

func countWords(content string) int {
	count := 0
	for _, value := range content {
		if !unicode.IsSpace(value) {
			count++
		}
	}
	return count
}

func mapWriteError(err error) error {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return apperror.Wrap(err, apperror.CodeConflict, http.StatusConflict, "该书已存在相同章节序号")
		case "23503", "23514":
			return apperror.Wrap(err, apperror.CodeInvalidArgument, http.StatusBadRequest, "章节关联或字段状态无效")
		}
	}
	return apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "保存章节失败")
}

func (service *Service) nextID(ctx context.Context) (int64, error) {
	var id int64
	err := service.db.QueryRowContext(ctx, `SELECT nextval(pg_get_serial_sequence('novel_chapters','id'))`).Scan(&id)
	return id, err
}

func (service *Service) Create(ctx context.Context, raw Input) (Chapter, error) {
	input, err := normalizeInput(raw, true)
	if err != nil {
		return Chapter{}, err
	}
	id, err := service.nextID(ctx)
	if err != nil {
		return Chapter{}, apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "分配章节 ID 失败")
	}
	text := *input.Content
	object, err := service.objects.UploadVerified(ctx, objectstore.Target{Kind: objectstore.KindChapterContent, BookID: input.BookID, OwnerID: id}, []byte(text), "text/plain; charset=utf-8")
	if err != nil {
		return Chapter{}, apperror.Wrap(err, apperror.CodeUnavailable, http.StatusServiceUnavailable, "保存章节正文失败")
	}
	err = service.objects.ActivateWithTx(ctx, object.ID, func(tx *sql.Tx, target objectstore.Target) error {
		if target.BookID != input.BookID || target.OwnerID != id {
			return errors.New("chapter object target mismatch")
		}
		if err := lockBook(ctx, tx, input.BookID); err != nil {
			return err
		}
		chapterNo, err := resolveChapterNo(ctx, tx, input.BookID, input.ChapterNo)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO novel_chapters
			(id,book_id,chapter_no,chapter_name,word_count,is_vip,book_price_coin,chapter_status,ai_clean_status,source_type)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'manual')`, id, input.BookID, chapterNo, input.ChapterName, countWords(text), input.IsVIP, input.BookPriceCoin, input.ChapterStatus, input.AICleanStatus)
		if err != nil {
			return mapWriteError(err)
		}
		return refreshBookStats(ctx, tx, input.BookID)
	})
	if err != nil {
		return Chapter{}, mapWriteError(err)
	}
	return service.Get(ctx, id)
}

func lockBook(ctx context.Context, tx *sql.Tx, bookID int64) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT true FROM novel_books WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, bookID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return invalid("书籍不存在")
	} else if err != nil {
		return err
	}
	return nil
}

func resolveChapterNo(ctx context.Context, tx *sql.Tx, bookID int64, requested *int) (int, error) {
	if requested != nil {
		return *requested, nil
	}
	var chapterNo int
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(max(chapter_no),-1)+1 FROM novel_chapters WHERE book_id=$1 AND deleted_at IS NULL`, bookID).Scan(&chapterNo)
	return chapterNo, err
}

func (service *Service) Update(ctx context.Context, id int64, raw Input) (Chapter, error) {
	input, err := normalizeInput(raw, false)
	if err != nil {
		return Chapter{}, err
	}
	current, err := service.Get(ctx, id)
	if err != nil {
		return Chapter{}, err
	}
	if input.BookID != current.BookID {
		return Chapter{}, invalid("章节不能直接移动到其他书籍")
	}
	if input.Content == nil {
		err = service.updateMetadata(ctx, id, current.BookID, input)
	} else {
		text := *input.Content
		object, uploadErr := service.objects.UploadVerified(ctx, objectstore.Target{Kind: objectstore.KindChapterContent, BookID: input.BookID, OwnerID: id}, []byte(text), "text/plain; charset=utf-8")
		if uploadErr != nil {
			return Chapter{}, apperror.Wrap(uploadErr, apperror.CodeUnavailable, http.StatusServiceUnavailable, "保存章节正文失败")
		}
		err = service.objects.ActivateWithTx(ctx, object.ID, func(tx *sql.Tx, target objectstore.Target) error {
			if target.BookID != input.BookID || target.OwnerID != id {
				return errors.New("chapter object target mismatch")
			}
			return updateChapterTx(ctx, tx, id, current.BookID, input, countWords(text))
		})
	}
	if err != nil {
		return Chapter{}, mapWriteError(err)
	}
	return service.Get(ctx, id)
}

func (service *Service) updateMetadata(ctx context.Context, id, oldBookID int64, input Input) error {
	tx, err := service.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := updateChapterTx(ctx, tx, id, oldBookID, input, -1); err != nil {
		return err
	}
	return tx.Commit()
}

func updateChapterTx(ctx context.Context, tx *sql.Tx, id, oldBookID int64, input Input, wordCount int) error {
	bookIDs := []int64{oldBookID}
	if input.BookID != oldBookID {
		bookIDs = append(bookIDs, input.BookID)
	}
	for _, bookID := range bookIDs {
		if err := lockBook(ctx, tx, bookID); err != nil {
			return err
		}
	}
	var currentNo, currentWordCount int
	if err := tx.QueryRowContext(ctx, `SELECT chapter_no,word_count FROM novel_chapters WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&currentNo, &currentWordCount); errors.Is(err, sql.ErrNoRows) {
		return chapterNotFound()
	} else if err != nil {
		return err
	}
	chapterNo := currentNo
	if input.ChapterNo != nil {
		chapterNo = *input.ChapterNo
	}
	if wordCount < 0 {
		wordCount = currentWordCount
	}
	result, err := tx.ExecContext(ctx, `UPDATE novel_chapters SET book_id=$2,chapter_no=$3,chapter_name=$4,word_count=$5,is_vip=$6,book_price_coin=$7,chapter_status=$8,ai_clean_status=$9,updated_at=now() WHERE id=$1 AND deleted_at IS NULL`, id, input.BookID, chapterNo, input.ChapterName, wordCount, input.IsVIP, input.BookPriceCoin, input.ChapterStatus, input.AICleanStatus)
	if err != nil {
		return mapWriteError(err)
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return chapterNotFound()
	}
	for _, bookID := range bookIDs {
		if err := refreshBookStats(ctx, tx, bookID); err != nil {
			return err
		}
	}
	return nil
}

func refreshBookStats(ctx context.Context, tx *sql.Tx, bookID int64) error {
	_, err := tx.ExecContext(ctx, `UPDATE novel_books b SET
		word_count=COALESCE((SELECT sum(c.word_count)::integer FROM novel_chapters c WHERE c.book_id=b.id AND c.deleted_at IS NULL AND c.chapter_status='enabled'),0),
		last_chapter_id=(SELECT c.id FROM novel_chapters c WHERE c.book_id=b.id AND c.deleted_at IS NULL AND c.chapter_status='enabled' ORDER BY c.chapter_no DESC,c.id DESC LIMIT 1),
		last_chapter_name=(SELECT c.chapter_name FROM novel_chapters c WHERE c.book_id=b.id AND c.deleted_at IS NULL AND c.chapter_status='enabled' ORDER BY c.chapter_no DESC,c.id DESC LIMIT 1),
		last_chapter_updated_at=(SELECT c.updated_at FROM novel_chapters c WHERE c.book_id=b.id AND c.deleted_at IS NULL AND c.chapter_status='enabled' ORDER BY c.chapter_no DESC,c.id DESC LIMIT 1),updated_at=now()
		WHERE b.id=$1 AND b.deleted_at IS NULL`, bookID)
	return err
}

func (service *Service) Get(ctx context.Context, id int64) (Chapter, error) {
	chapter, err := scanChapter(service.db.QueryRowContext(ctx, `SELECT id,book_id,chapter_no,chapter_name,word_count,is_vip,book_price_coin,chapter_status,ai_clean_status,source_type,created_at,updated_at FROM novel_chapters WHERE id=$1 AND deleted_at IS NULL`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Chapter{}, chapterNotFound()
	}
	if err != nil {
		return Chapter{}, apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "查询章节失败")
	}
	return chapter, nil
}

type rowScanner interface{ Scan(...any) error }

func scanChapter(row rowScanner) (Chapter, error) {
	var chapter Chapter
	err := row.Scan(&chapter.ID, &chapter.BookID, &chapter.ChapterNo, &chapter.ChapterName, &chapter.WordCount, &chapter.IsVIP, &chapter.BookPriceCoin, &chapter.ChapterStatus, &chapter.AICleanStatus, &chapter.SourceType, &chapter.CreatedAt, &chapter.UpdatedAt)
	return chapter, err
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
	args := []any{filter.BookID, "%" + strings.TrimSpace(filter.Keyword) + "%", strings.TrimSpace(filter.ChapterStatus), strings.TrimSpace(filter.AICleanStatus)}
	where := `deleted_at IS NULL AND ($1::bigint=0 OR book_id=$1) AND ($2='%%' OR chapter_name ILIKE $2) AND ($3='' OR chapter_status=$3) AND ($4='' OR ai_clean_status=$4)`
	var total int64
	if err := service.db.QueryRowContext(ctx, `SELECT count(*) FROM novel_chapters WHERE `+where, args...).Scan(&total); err != nil {
		return Page{}, err
	}
	rows, err := service.db.QueryContext(ctx, `SELECT id,book_id,chapter_no,chapter_name,word_count,is_vip,book_price_coin,chapter_status,ai_clean_status,source_type,created_at,updated_at FROM novel_chapters WHERE `+where+` ORDER BY book_id,chapter_no,id LIMIT $5 OFFSET $6`, append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)...)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	items := []Chapter{}
	for rows.Next() {
		chapter, err := scanChapter(rows)
		if err != nil {
			return Page{}, err
		}
		items = append(items, chapter)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	return Page{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func (service *Service) ReadContent(ctx context.Context, id int64) (Content, error) {
	chapter, err := service.Get(ctx, id)
	if err != nil {
		return Content{}, err
	}
	data, object, err := service.objects.ReadActive(ctx, objectstore.Target{Kind: objectstore.KindChapterContent, BookID: chapter.BookID, OwnerID: chapter.ID})
	if errors.Is(err, objectstore.ErrActiveObjectNotFound) {
		return Content{}, apperror.Wrap(err, apperror.CodeNotFound, http.StatusNotFound, "章节正文不存在")
	}
	if err != nil {
		return Content{}, apperror.Wrap(err, apperror.CodeUnavailable, http.StatusServiceUnavailable, "读取章节正文失败")
	}
	return Content{Chapter: chapter, Text: string(data), Version: object.Version, SHA256: object.SHA256, Bytes: object.ByteSize}, nil
}

func (service *Service) Delete(ctx context.Context, id int64) error {
	chapter, err := service.Get(ctx, id)
	if err != nil {
		return err
	}
	return service.objects.DeactivateWithTx(ctx, objectstore.Target{Kind: objectstore.KindChapterContent, BookID: chapter.BookID, OwnerID: id}, func(tx *sql.Tx, target objectstore.Target) error {
		if target.BookID != chapter.BookID || target.OwnerID != id {
			return errors.New("chapter object target mismatch")
		}
		if err := lockBook(ctx, tx, chapter.BookID); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `UPDATE novel_chapters SET deleted_at=now(),chapter_status='disabled',updated_at=now() WHERE id=$1 AND deleted_at IS NULL`, id)
		if err != nil {
			return err
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			return chapterNotFound()
		}
		return refreshBookStats(ctx, tx, chapter.BookID)
	})
}
