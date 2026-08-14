package adminfeedback

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

type SQLRepository struct{ DB *sql.DB }

const feedbackSelect = `SELECT f.id::text,f.reader_id::text,COALESCE(f.book_id::text,''),COALESCE(f.chapter_id::text,''),a.username,f.content,f.status,f.reply, f.created_at::text,COALESCE(f.replied_at::text,'') FROM reader_feedback f JOIN reader_accounts a ON a.id=f.reader_id`

func scan(row interface{ Scan(...any) error }, v *Feedback) error {
	return row.Scan(&v.ID, &v.ReaderID, &v.BookID, &v.ChapterID, &v.ReaderUsername, &v.Content, &v.Status, &v.Reply, &v.CreatedAt, &v.RepliedAt)
}
func (r SQLRepository) List(ctx context.Context, k, st string, p, n int) ([]Feedback, int64, error) {
	where := ` WHERE ($1='' OR a.username ILIKE '%'||$1||'%' OR f.content ILIKE '%'||$1||'%') AND ($2='' OR f.status=$2)`
	var total int64
	if err := r.DB.QueryRowContext(ctx, `SELECT count(*) FROM reader_feedback f JOIN reader_accounts a ON a.id=f.reader_id`+where, k, st).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.DB.QueryContext(ctx, feedbackSelect+where+` ORDER BY f.created_at DESC,f.id DESC LIMIT $3 OFFSET $4`, k, st, n, (p-1)*n)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]Feedback, 0)
	for rows.Next() {
		var v Feedback
		if err := scan(rows, &v); err != nil {
			return nil, 0, err
		}
		items = append(items, v)
	}
	return items, total, rows.Err()
}
func (r SQLRepository) Get(ctx context.Context, id int64) (Feedback, error) {
	var v Feedback
	err := scan(r.DB.QueryRowContext(ctx, feedbackSelect+` WHERE f.id=$1`, id), &v)
	return v, err
}
func (r SQLRepository) Reply(ctx context.Context, id int64, reply string) (Feedback, error) {
	reply = strings.TrimSpace(reply)
	if reply == "" || len([]rune(reply)) > 5000 {
		return Feedback{}, errors.New("invalid feedback reply")
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return Feedback{}, err
	}
	defer tx.Rollback()
	var v Feedback
	if err = scan(tx.QueryRowContext(ctx, feedbackSelect+` WHERE f.id=$1 AND f.status='pending' FOR UPDATE`, id), &v); err != nil {
		return Feedback{}, err
	}
	err = scan(tx.QueryRowContext(ctx, `UPDATE reader_feedback SET reply=$1,status='replied',replied_at=now(),updated_at=now() WHERE id=$2 RETURNING `+`id::text,reader_id::text,COALESCE(book_id::text,''),COALESCE(chapter_id::text,''),(SELECT username FROM reader_accounts WHERE id=reader_id),content,status,reply,created_at::text,COALESCE(replied_at::text,'')`, reply, id), &v)
	if err != nil {
		return Feedback{}, err
	}
	return v, tx.Commit()
}
