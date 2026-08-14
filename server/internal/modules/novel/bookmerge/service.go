package bookmerge

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/jackc/pgx/v5/pgconn"
)

const maxPageSize = 100

type Service struct {
	db      *sql.DB
	objects *objectstore.Service
}

type sourceSnapshot struct {
	bookID, importTaskID                                               int64
	bookName, authorName, sourceName, threadID, threadTitle, threadURL string
	sortTime                                                           time.Time
	sortTimeSource, oldPublishStatus                                   string
	order, chapterCount                                                int
}

type chapterSnapshot struct {
	sourceBookID, sourceChapterID, sourceObjectID int64
	sourceChapterNo, targetChapterNo              int
	sourceChapterName, targetChapterName          string
	contentSource, cleanResultID                  string
	wordCount                                     int
	sortTime                                      time.Time
	sortTimeSource                                string
	duplicate, excluded                           bool
	duplicateReason                               string
	content                                       []byte
	targetChapterID, targetObjectID               int64
}

func NewService(db *sql.DB, objects *objectstore.Service) *Service {
	return &Service{db: db, objects: objects}
}

func invalid(message string) error {
	return apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, message)
}

func parseIDs(raw []string, minimum int, label string) ([]int64, error) {
	seen := make(map[int64]struct{}, len(raw))
	ids := make([]int64, 0, len(raw))
	for _, value := range raw {
		id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil || id <= 0 {
			return nil, invalid(label + "必须是正整数字符串")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) < minimum {
		return nil, invalid(fmt.Sprintf("%s至少需要 %d 个", label, minimum))
	}
	return ids, nil
}

func placeholders(count int, start int) string {
	values := make([]string, count)
	for index := range values {
		values[index] = "$" + strconv.Itoa(start+index)
	}
	return strings.Join(values, ",")
}

func anyArgs(ids []int64) []any {
	args := make([]any, len(ids))
	for index, id := range ids {
		args[index] = id
	}
	return args
}

func (s *Service) EligibleBooks(ctx context.Context, keyword string) ([]EligibleBook, error) {
	keyword = strings.TrimSpace(keyword)
	rows, err := s.db.QueryContext(ctx, `SELECT b.id::text,b.book_name,b.author_name,
		(SELECT count(*)::text FROM novel_chapters c WHERE c.book_id=b.id AND c.deleted_at IS NULL),
		t.id::text,t.source_name,t.thread_title,COALESCE(t.thread_created_at::text,''),
		CASE WHEN t.thread_created_at IS NULL THEN 'import_task_created_at' ELSE 'thread_created_at' END
		FROM novel_books b
		JOIN LATERAL (
			SELECT i.id,i.source_name,i.thread_title,i.thread_created_at,i.created_at
			FROM novel_crawl_import_task i
			WHERE i.target_book_id=b.id AND i.status='succeeded'
			ORDER BY (i.import_mode='create') DESC,i.created_at,i.id LIMIT 1
		) t ON true
		WHERE b.deleted_at IS NULL AND b.source_type='forum_crawl'
		AND ($1='' OR b.book_name ILIKE '%'||$1||'%' OR b.author_name ILIKE '%'||$1||'%' OR t.thread_title ILIKE '%'||$1||'%')
		AND EXISTS(SELECT 1 FROM novel_chapters c WHERE c.book_id=b.id AND c.deleted_at IS NULL)
		AND NOT EXISTS(SELECT 1 FROM novel_book_merge_source ms JOIN novel_book_merge_task mt ON mt.id=ms.task_id WHERE ms.source_book_id=b.id AND mt.status='succeeded')
		ORDER BY COALESCE(t.thread_created_at,t.created_at),t.id,b.id LIMIT 100`, keyword)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []EligibleBook{}
	for rows.Next() {
		var item EligibleBook
		if err := rows.Scan(&item.BookID, &item.BookName, &item.AuthorName, &item.ChapterCount, &item.ImportTaskID, &item.SourceName, &item.ThreadTitle, &item.ThreadCreatedAt, &item.SortTimeSource); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) Preview(ctx context.Context, input PreviewInput) (Preview, error) {
	ids, err := parseIDs(input.SourceBookIDs, 2, "源书 ID")
	if err != nil {
		return Preview{}, err
	}
	sources, chapters, err := s.buildPreview(ctx, ids)
	if err != nil {
		return Preview{}, err
	}
	return renderPreview(sources, chapters), nil
}

func (s *Service) buildPreview(ctx context.Context, ids []int64) ([]sourceSnapshot, []chapterSnapshot, error) {
	args := anyArgs(ids)
	query := `SELECT b.id,b.book_name,b.author_name,b.publish_status,t.id,t.source_name,t.forum_thread_id,t.thread_title,t.thread_url,
		COALESCE(t.thread_created_at,t.created_at),
		CASE WHEN t.thread_created_at IS NULL THEN 'import_task_created_at' ELSE 'thread_created_at' END,
		(SELECT count(*) FROM novel_chapters c WHERE c.book_id=b.id AND c.deleted_at IS NULL)
		FROM novel_books b
		JOIN LATERAL (
			SELECT i.* FROM novel_crawl_import_task i
			WHERE i.target_book_id=b.id AND i.status='succeeded'
			ORDER BY (i.import_mode='create') DESC,i.created_at,i.id LIMIT 1
		) t ON true
		WHERE b.id IN (` + placeholders(len(ids), 1) + `) AND b.deleted_at IS NULL AND b.source_type='forum_crawl'
		AND NOT EXISTS(SELECT 1 FROM novel_book_merge_source ms JOIN novel_book_merge_task mt ON mt.id=ms.task_id WHERE ms.source_book_id=b.id AND mt.status='succeeded')`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	sources := make([]sourceSnapshot, 0, len(ids))
	for rows.Next() {
		var source sourceSnapshot
		if err := rows.Scan(&source.bookID, &source.bookName, &source.authorName, &source.oldPublishStatus, &source.importTaskID, &source.sourceName, &source.threadID, &source.threadTitle, &source.threadURL, &source.sortTime, &source.sortTimeSource, &source.chapterCount); err != nil {
			return nil, nil, err
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(sources) != len(ids) {
		return nil, nil, invalid("源书必须存在、由成功论坛导入产生、包含章节且未参与过成功合并")
	}
	sort.Slice(sources, func(i, j int) bool {
		if !sources[i].sortTime.Equal(sources[j].sortTime) {
			return sources[i].sortTime.Before(sources[j].sortTime)
		}
		if sources[i].importTaskID != sources[j].importTaskID {
			return sources[i].importTaskID < sources[j].importTaskID
		}
		return sources[i].bookID < sources[j].bookID
	})
	chapters := make([]chapterSnapshot, 0)
	for index := range sources {
		sources[index].order = index + 1
		chapterStart := len(chapters)
		chapterRows, queryErr := s.db.QueryContext(ctx, `SELECT c.id,c.chapter_no,c.chapter_name,r.object_id
			FROM novel_chapters c JOIN novel_object_references r
			ON r.object_kind='chapter_content' AND r.book_id=c.book_id AND r.owner_id=c.id
			WHERE c.book_id=$1 AND c.deleted_at IS NULL ORDER BY c.chapter_no,c.id`, sources[index].bookID)
		if queryErr != nil {
			return nil, nil, queryErr
		}
		for chapterRows.Next() {
			var chapter chapterSnapshot
			chapter.sourceBookID = sources[index].bookID
			chapter.sortTime = sources[index].sortTime
			chapter.sortTimeSource = sources[index].sortTimeSource
			chapter.contentSource = "original"
			if err := chapterRows.Scan(&chapter.sourceChapterID, &chapter.sourceChapterNo, &chapter.sourceChapterName, &chapter.sourceObjectID); err != nil {
				chapterRows.Close()
				return nil, nil, err
			}
			content, _, readErr := s.objects.ReadActive(ctx, objectstore.Target{Kind: objectstore.KindChapterContent, BookID: chapter.sourceBookID, OwnerID: chapter.sourceChapterID})
			if readErr != nil {
				chapterRows.Close()
				return nil, nil, apperror.Wrap(readErr, apperror.CodeUnavailable, http.StatusServiceUnavailable, "读取并校验源章节正文失败")
			}
			if !utf8.Valid(content) {
				chapterRows.Close()
				return nil, nil, invalid("源章节正文不是 UTF-8")
			}
			chapter.content = content
			chapter.targetChapterName = chapter.sourceChapterName
			chapter.wordCount = countWords(content)
			chapters = append(chapters, chapter)
		}
		if err := chapterRows.Close(); err != nil {
			return nil, nil, err
		}
		if len(chapters)-chapterStart != sources[index].chapterCount {
			return nil, nil, invalid("源书存在缺少活动正文对象的章节，不能合并")
		}
	}
	if len(chapters) == 0 {
		return nil, nil, invalid("源书没有可合并且具备活动正文对象的章节")
	}
	markDuplicates(chapters)
	for index := range chapters {
		chapters[index].targetChapterNo = index + 1
	}
	return sources, chapters, nil
}

func countWords(content []byte) int {
	count := 0
	for _, value := range string(content) {
		if !unicode.IsSpace(value) {
			count++
		}
	}
	return count
}

func normalizedTitle(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func contentDigest(content []byte) string {
	compact := strings.Join(strings.Fields(string(content)), "")
	digest := sha256.Sum256([]byte(compact))
	return hex.EncodeToString(digest[:])
}

func markDuplicates(chapters []chapterSnapshot) {
	titles := map[string]int{}
	contents := map[string]int{}
	for index := range chapters {
		title := normalizedTitle(chapters[index].targetChapterName)
		digest := contentDigest(chapters[index].content)
		reasons := []string{}
		if previous, exists := titles[title]; title != "" && exists {
			chapters[previous].duplicate = true
			chapters[previous].duplicateReason = appendReason(chapters[previous].duplicateReason, "标题重复")
			reasons = append(reasons, "标题重复")
		} else if title != "" {
			titles[title] = index
		}
		if previous, exists := contents[digest]; exists {
			chapters[previous].duplicate = true
			chapters[previous].duplicateReason = appendReason(chapters[previous].duplicateReason, "正文重复")
			reasons = append(reasons, "正文重复")
		} else {
			contents[digest] = index
		}
		if len(reasons) > 0 {
			chapters[index].duplicate = true
			chapters[index].duplicateReason = strings.Join(reasons, "、")
		}
	}
}

func appendReason(current, value string) string {
	if current == "" {
		return value
	}
	if strings.Contains(current, value) {
		return current
	}
	return current + "、" + value
}

func renderPreview(sources []sourceSnapshot, chapters []chapterSnapshot) Preview {
	result := Preview{Sources: make([]PreviewSource, 0, len(sources)), Chapters: make([]PreviewChapter, 0, len(chapters))}
	for _, source := range sources {
		result.Sources = append(result.Sources, PreviewSource{
			SourceBookID: strconv.FormatInt(source.bookID, 10), SourceBookName: source.bookName, SourceAuthorName: source.authorName,
			SourceImportTaskID: strconv.FormatInt(source.importTaskID, 10), SourceName: source.sourceName, SourceThreadID: source.threadID,
			SourceThreadTitle: source.threadTitle, SourceThreadURL: source.threadURL, SortTime: source.sortTime.Format(time.RFC3339Nano), SortTimeSource: source.sortTimeSource,
			SourceOrder: source.order, ChapterCount: strconv.Itoa(source.chapterCount), OldPublishStatus: source.oldPublishStatus,
		})
	}
	duplicates, clean, original := 0, 0, 0
	for _, chapter := range chapters {
		if chapter.duplicate {
			duplicates++
		}
		if chapter.contentSource == "clean_result" {
			clean++
		} else {
			original++
		}
		result.Chapters = append(result.Chapters, PreviewChapter{
			SourceBookID: strconv.FormatInt(chapter.sourceBookID, 10), SourceChapterID: strconv.FormatInt(chapter.sourceChapterID, 10),
			SourceChapterNo: chapter.sourceChapterNo, SourceChapterName: chapter.sourceChapterName, TargetChapterNo: chapter.targetChapterNo,
			TargetChapterName: chapter.targetChapterName, ContentSource: chapter.contentSource, CleanResultID: chapter.cleanResultID,
			WordCount: strconv.Itoa(chapter.wordCount), SortTime: chapter.sortTime.Format(time.RFC3339Nano), SortTimeSource: chapter.sortTimeSource,
			DuplicateFlag: chapter.duplicate, DuplicateReason: chapter.duplicateReason, Excluded: chapter.excluded,
		})
	}
	result.ChapterCount = strconv.Itoa(len(chapters))
	result.DuplicateChapterCount = strconv.Itoa(duplicates)
	result.CleanContentCount = strconv.Itoa(clean)
	result.OriginalContentCount = strconv.Itoa(original)
	return result
}

func normalizeTarget(raw TargetBookInput) (TargetBookInput, int64, error) {
	raw.BookName = strings.TrimSpace(raw.BookName)
	raw.AuthorID = strings.TrimSpace(raw.AuthorID)
	raw.CategoryCode = strings.TrimSpace(raw.CategoryCode)
	raw.Description = strings.TrimSpace(raw.Description)
	raw.BookStatus = strings.TrimSpace(raw.BookStatus)
	raw.PublishStatus = strings.TrimSpace(raw.PublishStatus)
	raw.OperatorName = strings.TrimSpace(raw.OperatorName)
	if raw.WorkDirection != nil {
		value := strings.TrimSpace(*raw.WorkDirection)
		if value == "" {
			raw.WorkDirection = nil
		} else {
			raw.WorkDirection = &value
		}
	}
	authorID, err := strconv.ParseInt(raw.AuthorID, 10, 64)
	if err != nil || authorID <= 0 {
		return raw, 0, invalid("作者 ID 必须是正整数字符串")
	}
	if raw.BookName == "" || len([]rune(raw.BookName)) > 100 || raw.CategoryCode == "" {
		return raw, 0, invalid("新书名称和主分类不能为空，书名不能超过 100 个字符")
	}
	if len([]rune(raw.Description)) > 2000 || len([]rune(raw.OperatorName)) > 64 {
		return raw, 0, invalid("新书简介或操作人超过字段长度限制")
	}
	if raw.BookStatus != "serializing" && raw.BookStatus != "completed" {
		return raw, 0, invalid("新书作品状态无效")
	}
	if raw.PublishStatus != "draft" && raw.PublishStatus != "published" {
		return raw, 0, invalid("新书发布状态必须是草稿或已发布")
	}
	return raw, authorID, nil
}

func (s *Service) Execute(ctx context.Context, input ExecuteInput) (Task, error) {
	ids, err := parseIDs(input.SourceBookIDs, 2, "源书 ID")
	if err != nil {
		return Task{}, err
	}
	excludedIDs, err := parseIDs(input.ExcludedChapterIDs, 0, "排除章节 ID")
	if err != nil && len(input.ExcludedChapterIDs) > 0 {
		return Task{}, err
	}
	target, authorID, err := normalizeTarget(input.TargetBook)
	if err != nil {
		return Task{}, err
	}
	sources, chapters, err := s.buildPreview(ctx, ids)
	if err != nil {
		return Task{}, err
	}
	excluded := make(map[int64]struct{}, len(excludedIDs))
	for _, id := range excludedIDs {
		excluded[id] = struct{}{}
	}
	includedCount := 0
	for index := range chapters {
		_, chapters[index].excluded = excluded[chapters[index].sourceChapterID]
		if !chapters[index].excluded {
			includedCount++
			chapters[index].targetChapterNo = includedCount
		}
	}
	if includedCount == 0 {
		return Task{}, invalid("至少保留一个章节")
	}
	if len(excluded) != len(chapters)-includedCount {
		return Task{}, invalid("排除章节必须属于本次合并预览")
	}
	duplicateCount := 0
	for _, chapter := range chapters {
		if chapter.duplicate {
			duplicateCount++
		}
	}
	var taskID int64
	err = s.db.QueryRowContext(ctx, `INSERT INTO novel_book_merge_task(target_book_name,status,source_count,chapter_count,included_chapter_count,excluded_chapter_count,duplicate_chapter_count,operator_name)
		VALUES($1,'running',$2,$3,$4,$5,$6,$7) RETURNING id`, target.BookName, len(sources), len(chapters), includedCount, len(chapters)-includedCount, duplicateCount, target.OperatorName).Scan(&taskID)
	if err != nil {
		return Task{}, err
	}
	fail := func(cause error) (Task, error) {
		message := cause.Error()
		if len([]rune(message)) > 1000 {
			message = string([]rune(message)[:1000])
		}
		_, _ = s.db.ExecContext(context.WithoutCancel(ctx), `UPDATE novel_book_merge_task SET status='failed',error_summary=$2,end_time=now(),updated_at=now() WHERE id=$1 AND status='running'`, taskID, message)
		return Task{}, cause
	}
	var targetBookID int64
	if err = s.db.QueryRowContext(ctx, `SELECT nextval(pg_get_serial_sequence('novel_books','id'))`).Scan(&targetBookID); err != nil {
		return fail(err)
	}
	objectIDs := make([]int64, 0, includedCount)
	abandonUploaded := func(cause error) error {
		if cleanupErr := s.objects.AbandonVerified(context.WithoutCancel(ctx), objectIDs); cleanupErr != nil {
			return errors.Join(cause, cleanupErr)
		}
		return cause
	}
	for index := range chapters {
		if chapters[index].excluded {
			continue
		}
		if err = s.db.QueryRowContext(ctx, `SELECT nextval(pg_get_serial_sequence('novel_chapters','id'))`).Scan(&chapters[index].targetChapterID); err != nil {
			return fail(abandonUploaded(err))
		}
		object, uploadErr := s.objects.UploadVerified(ctx, objectstore.Target{Kind: objectstore.KindChapterContent, BookID: targetBookID, OwnerID: chapters[index].targetChapterID}, chapters[index].content, "text/plain; charset=utf-8")
		if uploadErr != nil {
			return fail(abandonUploaded(apperror.Wrap(uploadErr, apperror.CodeUnavailable, http.StatusServiceUnavailable, "上传并校验合并章节正文失败")))
		}
		chapters[index].targetObjectID = object.ID
		objectIDs = append(objectIDs, object.ID)
	}
	err = s.objects.ActivateManyWithTx(ctx, objectIDs, func(tx *sql.Tx, targets []objectstore.Target) error {
		return s.executeTx(ctx, tx, taskID, targetBookID, authorID, target, sources, chapters, targets)
	})
	if err != nil {
		return fail(mapWriteError(err))
	}
	return s.Get(ctx, taskID)
}

func (s *Service) executeTx(ctx context.Context, tx *sql.Tx, taskID, targetBookID, authorID int64, target TargetBookInput, sources []sourceSnapshot, chapters []chapterSnapshot, objectTargets []objectstore.Target) error {
	if len(objectTargets) == 0 {
		return errors.New("merged book requires active chapter objects")
	}
	for _, source := range sources {
		var publishStatus string
		if err := tx.QueryRowContext(ctx, `SELECT publish_status FROM novel_books WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, source.bookID).Scan(&publishStatus); err != nil {
			return err
		}
		if publishStatus != source.oldPublishStatus {
			return errors.New("source book changed after preview")
		}
		var alreadyMerged bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
			SELECT 1 FROM novel_book_merge_source ms
			JOIN novel_book_merge_task mt ON mt.id=ms.task_id
			WHERE ms.source_book_id=$1 AND mt.status='succeeded'
		)`, source.bookID).Scan(&alreadyMerged); err != nil {
			return err
		}
		if alreadyMerged {
			return errors.New("source book was merged after preview")
		}
		var chapterCount int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM novel_chapters WHERE book_id=$1 AND deleted_at IS NULL`, source.bookID).Scan(&chapterCount); err != nil {
			return err
		}
		if chapterCount != source.chapterCount {
			return errors.New("source book chapter count changed after preview")
		}
	}
	for _, chapter := range chapters {
		var activeObjectID int64
		var chapterNo int
		var chapterName string
		if err := tx.QueryRowContext(ctx, `SELECT c.chapter_no,c.chapter_name,r.object_id FROM novel_chapters c JOIN novel_object_references r ON r.object_kind='chapter_content' AND r.book_id=c.book_id AND r.owner_id=c.id WHERE c.id=$1 AND c.book_id=$2 AND c.deleted_at IS NULL`, chapter.sourceChapterID, chapter.sourceBookID).Scan(&chapterNo, &chapterName, &activeObjectID); err != nil {
			return err
		}
		if chapterNo != chapter.sourceChapterNo || chapterName != chapter.sourceChapterName || activeObjectID != chapter.sourceObjectID {
			return errors.New("source chapter changed after preview")
		}
	}
	var categoryID int64
	var categoryName, authorName string
	if err := tx.QueryRowContext(ctx, `SELECT id,name FROM novel_categories WHERE kind='primary' AND code=$1 AND enabled AND deleted_at IS NULL`, target.CategoryCode).Scan(&categoryID, &categoryName); err != nil {
		return invalid("主分类不存在或已停用")
	}
	if err := tx.QueryRowContext(ctx, `SELECT pen_name FROM novel_authors WHERE id=$1 AND status='active' AND deleted_at IS NULL`, authorID).Scan(&authorName); err != nil {
		return invalid("作者不存在或状态不可用")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO novel_books(id,work_direction,primary_category_id,category_code,category_name,book_name,author_id,author_name,description,score,book_status,publish_status,source_type,charge_mode)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,0,$10,$11,'book_merge','login_free')`, targetBookID, target.WorkDirection, categoryID, target.CategoryCode, categoryName, target.BookName, authorID, authorName, target.Description, target.BookStatus, target.PublishStatus); err != nil {
		return err
	}
	for _, chapter := range chapters {
		if chapter.excluded {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO novel_chapters(id,book_id,chapter_no,chapter_name,word_count,chapter_status,ai_clean_status,source_type)
			VALUES($1,$2,$3,$4,$5,'enabled','pending','book_merge')`, chapter.targetChapterID, targetBookID, chapter.targetChapterNo, chapter.targetChapterName, chapter.wordCount); err != nil {
			return err
		}
	}
	for _, source := range sources {
		if _, err := tx.ExecContext(ctx, `INSERT INTO novel_book_merge_source(task_id,source_book_id,source_book_name,source_author_name,source_import_task_id,source_name,source_thread_id,source_thread_title,source_thread_url,sort_time,sort_time_source,source_order,old_publish_status,archived_publish_status,archive_time)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::timestamptz,$11,$12,$13,'draft',now())`, taskID, source.bookID, source.bookName, source.authorName, source.importTaskID, source.sourceName, source.threadID, source.threadTitle, source.threadURL, source.sortTime, source.sortTimeSource, source.order, source.oldPublishStatus); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE novel_books SET publish_status='draft',updated_at=now() WHERE id=$1`, source.bookID); err != nil {
			return err
		}
	}
	for _, chapter := range chapters {
		var targetBook any
		var targetChapter, targetNo, targetObject any
		if !chapter.excluded {
			targetBook, targetChapter, targetNo, targetObject = targetBookID, chapter.targetChapterID, chapter.targetChapterNo, chapter.targetObjectID
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO novel_book_merge_chapter(task_id,source_book_id,source_chapter_id,source_chapter_no,source_chapter_name,source_object_id,target_book_id,target_chapter_id,target_chapter_no,target_chapter_name,target_object_id,content_source,clean_result_id,sort_time,sort_time_source,duplicate_flag,duplicate_reason,excluded,exclude_reason)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NULLIF($13,'')::bigint,$14::timestamptz,$15,$16,$17,$18,$19)`, taskID, chapter.sourceBookID, chapter.sourceChapterID, chapter.sourceChapterNo, chapter.sourceChapterName, chapter.sourceObjectID, targetBook, targetChapter, targetNo, nullableString(chapter.targetChapterName, chapter.excluded), targetObject, chapter.contentSource, chapter.cleanResultID, chapter.sortTime, chapter.sortTimeSource, chapter.duplicate, chapter.duplicateReason, chapter.excluded, excludeReason(chapter.excluded)); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE novel_books b SET word_count=COALESCE((SELECT sum(c.word_count)::integer FROM novel_chapters c WHERE c.book_id=b.id AND c.deleted_at IS NULL AND c.chapter_status='enabled'),0),last_chapter_id=(SELECT c.id FROM novel_chapters c WHERE c.book_id=b.id AND c.deleted_at IS NULL AND c.chapter_status='enabled' ORDER BY c.chapter_no DESC,c.id DESC LIMIT 1),last_chapter_name=(SELECT c.chapter_name FROM novel_chapters c WHERE c.book_id=b.id AND c.deleted_at IS NULL AND c.chapter_status='enabled' ORDER BY c.chapter_no DESC,c.id DESC LIMIT 1),last_chapter_updated_at=(SELECT c.updated_at FROM novel_chapters c WHERE c.book_id=b.id AND c.deleted_at IS NULL AND c.chapter_status='enabled' ORDER BY c.chapter_no DESC,c.id DESC LIMIT 1),updated_at=now() WHERE b.id=$1`, targetBookID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE novel_book_merge_task SET target_book_id=$2,status='succeeded',end_time=now(),updated_at=now() WHERE id=$1 AND status='running'`, taskID, targetBookID)
	return err
}

func nullableString(value string, null bool) any {
	if null {
		return nil
	}
	return value
}

func excludeReason(excluded bool) string {
	if excluded {
		return "operator_excluded"
	}
	return ""
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
			return apperror.Wrap(err, apperror.CodeConflict, http.StatusConflict, "目标书已存在或源书已被合并")
		case "23503", "23514":
			return apperror.Wrap(err, apperror.CodeInvalidArgument, http.StatusBadRequest, "书籍合并关联或状态无效")
		}
	}
	return apperror.Wrap(err, apperror.CodeInternal, http.StatusInternalServerError, "执行书籍合并失败")
}

const taskColumns = `id::text,COALESCE(target_book_id::text,''),target_book_name,status,source_count::text,chapter_count::text,included_chapter_count::text,excluded_chapter_count::text,duplicate_chapter_count::text,content_policy,sort_policy,source_archive_mode,operator_name,error_summary,start_time::text,COALESCE(end_time::text,''),created_at::text,updated_at::text`

func scanTask(row interface{ Scan(...any) error }, task *Task) error {
	return row.Scan(&task.ID, &task.TargetBookID, &task.TargetBookName, &task.Status, &task.SourceCount, &task.ChapterCount, &task.IncludedChapterCount, &task.ExcludedChapterCount, &task.DuplicateChapterCount, &task.ContentPolicy, &task.SortPolicy, &task.SourceArchiveMode, &task.OperatorName, &task.ErrorSummary, &task.StartTime, &task.EndTime, &task.CreatedAt, &task.UpdatedAt)
}

func (s *Service) List(ctx context.Context, keyword, status string, page, pageSize int) (Page, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	keyword, status = strings.TrimSpace(keyword), strings.TrimSpace(status)
	where := ` WHERE ($1='' OR target_book_name ILIKE '%'||$1||'%' OR target_book_id::text=$1) AND ($2='' OR status=$2)`
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM novel_book_merge_task`+where, keyword, status).Scan(&total); err != nil {
		return Page{}, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+taskColumns+` FROM novel_book_merge_task`+where+` ORDER BY created_at DESC,id DESC LIMIT $3 OFFSET $4`, keyword, status, pageSize, (page-1)*pageSize)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	items := []Task{}
	for rows.Next() {
		var task Task
		if err := scanTask(rows, &task); err != nil {
			return Page{}, err
		}
		items = append(items, task)
	}
	return Page{Items: items, Total: total, Page: page, PageSize: pageSize}, rows.Err()
}

func (s *Service) Get(ctx context.Context, id int64) (Task, error) {
	if id <= 0 {
		return Task{}, invalid("合并任务 ID 必须是正整数")
	}
	var task Task
	if err := scanTask(s.db.QueryRowContext(ctx, `SELECT `+taskColumns+` FROM novel_book_merge_task WHERE id=$1`, id), &task); errors.Is(err, sql.ErrNoRows) {
		return Task{}, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "合并任务不存在")
	} else if err != nil {
		return Task{}, err
	}
	sourceRows, err := s.db.QueryContext(ctx, `SELECT id::text,source_book_id::text,source_book_name,source_author_name,source_import_task_id::text,source_name,source_thread_id,source_thread_title,sort_time::text,sort_time_source,source_order::text,old_publish_status,archived_publish_status,archive_time::text FROM novel_book_merge_source WHERE task_id=$1 ORDER BY source_order,id`, id)
	if err != nil {
		return Task{}, err
	}
	defer sourceRows.Close()
	task.Sources = []TaskSource{}
	for sourceRows.Next() {
		var source TaskSource
		if err := sourceRows.Scan(&source.ID, &source.SourceBookID, &source.SourceBookName, &source.SourceAuthorName, &source.SourceImportTaskID, &source.SourceName, &source.SourceThreadID, &source.SourceThreadTitle, &source.SortTime, &source.SortTimeSource, &source.SourceOrder, &source.OldPublishStatus, &source.ArchivedPublishStatus, &source.ArchiveTime); err != nil {
			return Task{}, err
		}
		task.Sources = append(task.Sources, source)
	}
	chapterRows, err := s.db.QueryContext(ctx, `SELECT id::text,source_book_id::text,source_chapter_id::text,source_chapter_no::text,source_chapter_name,COALESCE(target_book_id::text,''),COALESCE(target_chapter_id::text,''),COALESCE(target_chapter_no::text,''),COALESCE(target_chapter_name,''),content_source,duplicate_flag,duplicate_reason,excluded,exclude_reason FROM novel_book_merge_chapter WHERE task_id=$1 ORDER BY excluded,target_chapter_no NULLS LAST,source_chapter_no,id`, id)
	if err != nil {
		return Task{}, err
	}
	defer chapterRows.Close()
	task.Chapters = []TaskChapter{}
	for chapterRows.Next() {
		var chapter TaskChapter
		if err := chapterRows.Scan(&chapter.ID, &chapter.SourceBookID, &chapter.SourceChapterID, &chapter.SourceChapterNo, &chapter.SourceChapterName, &chapter.TargetBookID, &chapter.TargetChapterID, &chapter.TargetChapterNo, &chapter.TargetChapterName, &chapter.ContentSource, &chapter.DuplicateFlag, &chapter.DuplicateReason, &chapter.Excluded, &chapter.ExcludeReason); err != nil {
			return Task{}, err
		}
		task.Chapters = append(task.Chapters, chapter)
	}
	return task, nil
}
