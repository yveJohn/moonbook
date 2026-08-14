package me

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (r SQLRepository) ListBookshelf(ctx context.Context, readerID int64) ([]Bookshelf, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT e.id,b.id,b.book_name,b.author_name,e.last_chapter_id,c.chapter_name,e.last_read_at FROM reader_bookshelf_entries e JOIN novel_books b ON b.id=e.book_id AND b.deleted_at IS NULL AND b.publish_status='published' LEFT JOIN novel_chapters c ON c.id=e.last_chapter_id AND c.book_id=e.book_id AND c.deleted_at IS NULL AND c.chapter_status='enabled' WHERE e.reader_id=$1 ORDER BY e.last_read_at DESC NULLS LAST,e.id DESC`, readerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Bookshelf, 0)
	for rows.Next() {
		var v Bookshelf
		var cid sql.NullInt64
		var cn sql.NullString
		var at sql.NullTime
		if err = rows.Scan(&v.ID, &v.BookID, &v.BookName, &v.AuthorName, &cid, &cn, &at); err != nil {
			return nil, err
		}
		if cid.Valid {
			v.LastChapterID = &cid.Int64
		}
		if cn.Valid {
			v.LastChapterName = &cn.String
		}
		if at.Valid {
			v.LastReadAt = &at.Time
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r SQLRepository) AddBookshelf(ctx context.Context, readerID, bookID int64) (Bookshelf, error) {
	var id int64
	err := r.DB.QueryRowContext(ctx, `INSERT INTO reader_bookshelf_entries(reader_id,book_id) SELECT $1,b.id FROM novel_books b WHERE b.id=$2 AND b.deleted_at IS NULL AND b.publish_status='published' ON CONFLICT(reader_id,book_id) DO UPDATE SET updated_at=reader_bookshelf_entries.updated_at RETURNING id`, readerID, bookID).Scan(&id)
	if err != nil {
		return Bookshelf{}, err
	}
	var v Bookshelf
	var cid sql.NullInt64
	var cn sql.NullString
	var at sql.NullTime
	err = r.DB.QueryRowContext(ctx, `SELECT e.id,b.id,b.book_name,b.author_name,e.last_chapter_id,c.chapter_name,e.last_read_at FROM reader_bookshelf_entries e JOIN novel_books b ON b.id=e.book_id AND b.publish_status='published' AND b.deleted_at IS NULL LEFT JOIN novel_chapters c ON c.id=e.last_chapter_id AND c.book_id=e.book_id AND c.deleted_at IS NULL AND c.chapter_status='enabled' WHERE e.id=$1 AND e.reader_id=$2`, id, readerID).Scan(&v.ID, &v.BookID, &v.BookName, &v.AuthorName, &cid, &cn, &at)
	if err != nil {
		return Bookshelf{}, err
	}
	if cid.Valid {
		v.LastChapterID = &cid.Int64
	}
	if cn.Valid {
		v.LastChapterName = &cn.String
	}
	if at.Valid {
		v.LastReadAt = &at.Time
	}
	return v, nil
}
func (r SQLRepository) RemoveBookshelf(ctx context.Context, readerID, bookID int64) (bool, error) {
	res, e := r.DB.ExecContext(ctx, `DELETE FROM reader_bookshelf_entries WHERE reader_id=$1 AND book_id=$2`, readerID, bookID)
	if e != nil {
		return false, e
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r SQLRepository) ListLikes(ctx context.Context, readerID int64) ([]LikedBook, error) {
	rows, e := r.DB.QueryContext(ctx, `SELECT l.id,b.id,b.book_name,b.author_name,b.description,b.category_code,b.category_name,b.word_count,b.like_count,l.created_at FROM reader_book_likes l JOIN novel_books b ON b.id=l.book_id AND b.publish_status='published' AND b.deleted_at IS NULL WHERE l.reader_id=$1 ORDER BY l.created_at DESC,l.id DESC`, readerID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := make([]LikedBook, 0)
	for rows.Next() {
		var v LikedBook
		if e = rows.Scan(&v.BookLikeID, &v.BookID, &v.BookName, &v.AuthorName, &v.Description, &v.CategoryCode, &v.CategoryName, &v.WordCount, &v.LikeCount, &v.LikedAt); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r SQLRepository) Like(ctx context.Context, readerID, bookID int64) (BookLike, error) {
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return BookLike{}, e
	}
	defer tx.Rollback()
	var id int64
	var count int
	e = tx.QueryRowContext(ctx, `SELECT like_count FROM novel_books WHERE id=$1 AND publish_status='published' AND deleted_at IS NULL FOR UPDATE`, bookID).Scan(&count)
	if e != nil {
		return BookLike{}, e
	}
	e = tx.QueryRowContext(ctx, `INSERT INTO reader_book_likes(reader_id,book_id) VALUES($1,$2) ON CONFLICT(reader_id,book_id) DO UPDATE SET book_id=EXCLUDED.book_id RETURNING id`, readerID, bookID).Scan(&id)
	if e != nil {
		return BookLike{}, e
	}
	// Count is derived from the row transition under the book lock, so retries do not double increment.
	// Reconcile from the relation rows while the book lock is held. This makes
	// retries and concurrent like/unlike operations converge to the exact count.
	if _, e = tx.ExecContext(ctx, `UPDATE novel_books SET like_count=(SELECT count(*) FROM reader_book_likes WHERE book_id=$1),updated_at=now() WHERE id=$1`, bookID); e != nil {
		return BookLike{}, e
	}
	e = tx.QueryRowContext(ctx, `SELECT like_count FROM novel_books WHERE id=$1`, bookID).Scan(&count)
	if e != nil {
		return BookLike{}, e
	}
	if e = tx.Commit(); e != nil {
		return BookLike{}, e
	}
	return BookLike{ID: id, BookID: bookID, Liked: true, LikeCount: count}, nil
}
func (r SQLRepository) Unlike(ctx context.Context, readerID, bookID int64) (BookLike, error) {
	tx, e := r.DB.BeginTx(ctx, nil)
	if e != nil {
		return BookLike{}, e
	}
	defer tx.Rollback()
	var count int
	if e = tx.QueryRowContext(ctx, `SELECT like_count FROM novel_books WHERE id=$1 AND publish_status='published' AND deleted_at IS NULL FOR UPDATE`, bookID).Scan(&count); e != nil {
		return BookLike{}, e
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM reader_book_likes WHERE reader_id=$1 AND book_id=$2`, readerID, bookID); e != nil {
		return BookLike{}, e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE novel_books SET like_count=(SELECT count(*) FROM reader_book_likes WHERE book_id=$1),updated_at=now() WHERE id=$1`, bookID); e != nil {
		return BookLike{}, e
	}
	if e = tx.QueryRowContext(ctx, `SELECT like_count FROM novel_books WHERE id=$1`, bookID).Scan(&count); e != nil {
		return BookLike{}, e
	}
	if e = tx.Commit(); e != nil {
		return BookLike{}, e
	}
	return BookLike{BookID: bookID, Liked: false, LikeCount: count}, nil
}

func (r SQLRepository) CreateFeedback(ctx context.Context, readerID int64, content string) (Feedback, error) {
	var v Feedback
	var reply string
	e := r.DB.QueryRowContext(ctx, `INSERT INTO reader_feedback(reader_id,content) VALUES($1,$2) RETURNING id,content,status,reply,replied_at,created_at`, readerID, content).Scan(&v.ID, &v.Content, &v.Status, &reply, &v.RepliedAt, &v.CreatedAt)
	v.Reply = &reply
	return v, e
}
func (r SQLRepository) ListFeedbacks(ctx context.Context, readerID int64, page, size int) ([]Feedback, int64, error) {
	var total int64
	if e := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM reader_feedback WHERE reader_id=$1`, readerID).Scan(&total); e != nil {
		return nil, 0, e
	}
	rows, e := r.DB.QueryContext(ctx, `SELECT id,content,status,reply,replied_at,created_at FROM reader_feedback WHERE reader_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, readerID, size, (page-1)*size)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	out := make([]Feedback, 0)
	for rows.Next() {
		var v Feedback
		var reply string
		if e = rows.Scan(&v.ID, &v.Content, &v.Status, &reply, &v.RepliedAt, &v.CreatedAt); e != nil {
			return nil, 0, e
		}
		v.Reply = &reply
		out = append(out, v)
	}
	return out, total, rows.Err()
}

func (r SQLRepository) ListHistory(ctx context.Context, readerID int64) ([]History, error) {
	rows, e := r.DB.QueryContext(ctx, `SELECT h.id,h.book_id,h.chapter_id,h.chapter_no,c.chapter_name,h.position_type,h.position_value,h.progress_percent,h.last_read_at FROM reader_reading_history h JOIN novel_books b ON b.id=h.book_id AND b.publish_status='published' AND b.deleted_at IS NULL JOIN novel_chapters c ON c.id=h.chapter_id AND c.book_id=h.book_id AND c.chapter_status='enabled' AND c.deleted_at IS NULL WHERE h.reader_id=$1 ORDER BY h.last_read_at DESC,h.id DESC`, readerID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	return scanHistory(rows)
}
func scanHistory(rows *sql.Rows) ([]History, error) {
	out := make([]History, 0)
	for rows.Next() {
		var v History
		if e := rows.Scan(&v.ID, &v.BookID, &v.ChapterID, &v.ChapterNo, &v.ChapterName, &v.PositionType, &v.PositionValue, &v.ProgressPercent, &v.LastReadAt); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r SQLRepository) GetHistory(ctx context.Context, readerID, bookID int64) (*History, error) {
	var v History
	e := r.DB.QueryRowContext(ctx, `SELECT h.id,h.book_id,h.chapter_id,h.chapter_no,c.chapter_name,h.position_type,h.position_value,h.progress_percent,h.last_read_at FROM reader_reading_history h JOIN novel_books b ON b.id=h.book_id AND b.publish_status='published' AND b.deleted_at IS NULL JOIN novel_chapters c ON c.id=h.chapter_id AND c.book_id=h.book_id AND c.chapter_status='enabled' AND c.deleted_at IS NULL WHERE h.reader_id=$1 AND h.book_id=$2`, readerID, bookID).Scan(&v.ID, &v.BookID, &v.ChapterID, &v.ChapterNo, &v.ChapterName, &v.PositionType, &v.PositionValue, &v.ProgressPercent, &v.LastReadAt)
	if e == sql.ErrNoRows {
		return nil, nil
	}
	return &v, e
}
func (r SQLRepository) UpdateHistory(ctx context.Context, readerID, bookID int64, in HistoryInput) (History, error) {
	var v History
	chapterNo := 0
	if in.ChapterNo != nil {
		chapterNo = *in.ChapterNo
	}
	e := r.DB.QueryRowContext(ctx, `INSERT INTO reader_reading_history(reader_id,book_id,chapter_id,chapter_no,position_type,position_value,progress_percent,last_read_at,updated_at) SELECT $1,b.id,c.id,CASE WHEN $3=0 THEN c.chapter_no ELSE $3 END,$4,$5,$6,now(),now() FROM novel_books b JOIN novel_chapters c ON c.book_id=b.id AND c.id=$2 AND c.deleted_at IS NULL AND c.chapter_status='enabled' WHERE b.id=$7 AND b.deleted_at IS NULL AND b.publish_status='published' ON CONFLICT(reader_id,book_id) DO UPDATE SET chapter_id=EXCLUDED.chapter_id,chapter_no=EXCLUDED.chapter_no,position_type=EXCLUDED.position_type,position_value=EXCLUDED.position_value,progress_percent=EXCLUDED.progress_percent,last_read_at=now(),updated_at=now() RETURNING id,book_id,chapter_id,chapter_no,position_type,position_value,progress_percent,last_read_at`, readerID, in.ChapterID, chapterNo, in.PositionType, in.PositionValue, in.ProgressPercent, bookID).Scan(&v.ID, &v.BookID, &v.ChapterID, &v.ChapterNo, &v.PositionType, &v.PositionValue, &v.ProgressPercent, &v.LastReadAt)
	if e != nil {
		return v, e
	}
	e = r.DB.QueryRowContext(ctx, `SELECT chapter_name FROM novel_chapters WHERE id=$1`, v.ChapterID).Scan(&v.ChapterName)
	return v, e
}
func (r SQLRepository) GetPreference(ctx context.Context, readerID int64) (*Preference, error) {
	var v Preference
	e := r.DB.QueryRowContext(ctx, `SELECT id,font_size,line_height,theme,reading_mode FROM reader_reading_preferences WHERE reader_id=$1`, readerID).Scan(&v.ID, &v.FontSize, &v.LineHeight, &v.Theme, &v.ReadingMode)
	if e == sql.ErrNoRows {
		return nil, nil
	}
	return &v, e
}
func (r SQLRepository) UpdatePreference(ctx context.Context, readerID int64, in PreferenceInput) (Preference, error) {
	var v Preference
	fs := 20
	if in.FontSize != nil {
		fs = *in.FontSize
	}
	e := r.DB.QueryRowContext(ctx, `INSERT INTO reader_reading_preferences(reader_id,font_size,line_height,theme,reading_mode) VALUES($1,$2,$3,$4,$5) ON CONFLICT(reader_id) DO UPDATE SET font_size=EXCLUDED.font_size,line_height=EXCLUDED.line_height,theme=EXCLUDED.theme,reading_mode=EXCLUDED.reading_mode,updated_at=now() RETURNING id,font_size,line_height,theme,reading_mode`, readerID, fs, in.LineHeight, in.Theme, in.ReadingMode).Scan(&v.ID, &v.FontSize, &v.LineHeight, &v.Theme, &v.ReadingMode)
	return v, e
}

func notFound(name string) error { return fmt.Errorf("%s not found", name) }

var _ = time.Now
