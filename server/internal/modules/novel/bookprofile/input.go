package bookprofile

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
)

type inputBuilder struct {
	db      *sql.DB
	objects *objectstore.Service
}

func (b inputBuilder) build(ctx context.Context, bookID int64, budget int) (string, string, json.RawMessage, BookSnapshot, error) {
	if budget < 1000 {
		budget = 60000
	}
	var name, categoryCode, categoryName, desc string
	if err := b.db.QueryRowContext(ctx, `SELECT book_name,category_code,category_name,description FROM novel_books WHERE id=$1 AND deleted_at IS NULL AND publish_status<>'deprecated'`, bookID).Scan(&name, &categoryCode, &categoryName, &desc); err != nil {
		return "", "", nil, BookSnapshot{}, fmt.Errorf("查询作品失败: %w", err)
	}
	original := BookSnapshot{BookName: name, CategoryCode: categoryCode, CategoryName: categoryName, BookDesc: desc, SubCategories: []SnapshotCategory{}}
	rows, err := b.db.QueryContext(ctx, `SELECT category_code,category_name,sort FROM novel_book_sub_categories WHERE book_id=$1 ORDER BY sort,category_id`, bookID)
	if err != nil {
		return "", "", nil, original, err
	}
	for rows.Next() {
		var c SnapshotCategory
		if err = rows.Scan(&c.Code, &c.Name, &c.Sort); err != nil {
			rows.Close()
			return "", "", nil, original, err
		}
		original.SubCategories = append(original.SubCategories, c)
	}
	rows.Close()
	candidates, err := b.categoryCandidates(ctx)
	if err != nil {
		return "", "", nil, original, err
	}
	type chapter struct {
		ID                       int64
		No                       int
		Name, CleanName, Summary string
		ObjectID                 sql.NullInt64
	}
	chapterRows, err := b.db.QueryContext(ctx, `SELECT c.id,c.chapter_no,c.chapter_name,r.cleaned_chapter_name,r.chapter_summary,r.cleaned_object_id FROM novel_chapters c LEFT JOIN novel_chapter_clean_result r ON r.chapter_id=c.id AND r.active AND r.status='success' WHERE c.book_id=$1 AND c.deleted_at IS NULL ORDER BY c.chapter_no,c.id`, bookID)
	if err != nil {
		return "", "", nil, original, err
	}
	chapters := []chapter{}
	for chapterRows.Next() {
		var c chapter
		if err = chapterRows.Scan(&c.ID, &c.No, &c.Name, &c.CleanName, &c.Summary, &c.ObjectID); err != nil {
			chapterRows.Close()
			return "", "", nil, original, err
		}
		chapters = append(chapters, c)
	}
	chapterRows.Close()
	base := map[string]any{"book_id": fmt.Sprint(bookID), "book": original, "category_candidates": candidates}
	cleaned := []map[string]any{}
	cleanedFits := true
	for _, c := range chapters {
		if !c.ObjectID.Valid {
			continue
		}
		data, _, readErr := b.objects.ReadStored(ctx, c.ObjectID.Int64)
		if readErr != nil {
			continue
		}
		cleaned = append(cleaned, map[string]any{"chapter_id": fmt.Sprint(c.ID), "chapter_no": c.No, "chapter_name": c.Name, "cleaned_chapter_name": c.CleanName, "cleaned_text": string(data)})
		payload := cloneMap(base)
		payload["input_mode"] = "sampled_cleaned_text"
		payload["chapters"] = cleaned
		if utf8.RuneCount(marshal(payload)) > budget {
			cleanedFits = false
			break
		}
	}
	if len(cleaned) > 0 && cleanedFits {
		payload := cloneMap(base)
		payload["input_mode"] = "sampled_cleaned_text"
		payload["chapters"] = cleaned
		if raw := marshal(payload); utf8.RuneCount(raw) <= budget {
			return finishInput("sampled_cleaned_text", raw, original)
		}
	}
	summaries := []map[string]any{}
	for _, c := range chapters {
		if c.Summary != "" {
			summaries = append(summaries, map[string]any{"chapter_id": fmt.Sprint(c.ID), "chapter_no": c.No, "chapter_name": c.Name, "cleaned_chapter_name": c.CleanName, "chapter_summary": c.Summary})
		}
	}
	payload := cloneMap(base)
	payload["input_mode"] = "chapter_summary"
	payload["chapters"] = summaries
	raw := marshal(payload)
	if utf8.RuneCount(raw) <= budget {
		return finishInput("chapter_summary", raw, original)
	}
	selected := []map[string]any{}
	payload = cloneMap(base)
	payload["input_mode"] = "chunk_summary"
	for _, item := range summaries {
		trial := append(append([]map[string]any{}, selected...), item)
		payload["summaries"] = trial
		if utf8.RuneCount(marshal(payload)) > budget {
			break
		}
		selected = trial
	}
	payload["summaries"] = selected
	payload["truncated"] = len(selected) < len(summaries)
	raw = marshal(payload)
	if utf8.RuneCount(raw) > budget {
		return "", "", nil, original, fmt.Errorf("AI 作品资料输入预算过小")
	}
	return finishInput("chunk_summary", raw, original)
}
func (b inputBuilder) categoryCandidates(ctx context.Context) ([]SnapshotCategory, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT code,name,sort FROM novel_categories WHERE enabled AND deleted_at IS NULL ORDER BY kind,sort,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []SnapshotCategory{}
	for rows.Next() {
		var c SnapshotCategory
		if err = rows.Scan(&c.Code, &c.Name, &c.Sort); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Sort < items[j].Sort })
	return items, rows.Err()
}
func cloneMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for k, v := range value {
		result[k] = v
	}
	return result
}
func marshal(value any) json.RawMessage { data, _ := json.Marshal(value); return data }
func finishInput(mode string, raw json.RawMessage, original BookSnapshot) (string, string, json.RawMessage, BookSnapshot, error) {
	sum := sha256.Sum256(raw)
	return mode, hex.EncodeToString(sum[:]), raw, original, nil
}
