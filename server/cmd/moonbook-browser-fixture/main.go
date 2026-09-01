package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	fixtureConfirmation   = "moonbook_browser"
	fixtureBookName       = "M3 浏览器验收作品"
	fixtureAuthorName     = "M3 浏览器验收作者"
	fixtureCategoryCode   = "m3-browser"
	fixtureInviteCode     = "M3-BROWSER-20260815"
	fixtureReaderPrefix   = "m3-browser-"
	fixtureCallbackPrefix = "m4-browser-callback-"
)

type config struct {
	dsn, endpoint, accessKey, secretKey, bucket, confirmation string
}

type cleanupStatement struct {
	query string
	args  []any
}

func main() { os.Exit(run(os.Args[1:], os.LookupEnv)) }

func run(args []string, lookup func(string) (string, bool)) int {
	if len(args) != 1 || (args[0] != "seed" && args[0] != "cleanup") {
		fmt.Fprintln(os.Stderr, "usage: moonbook-browser-fixture [seed|cleanup]")
		return 2
	}
	cfg := loadConfig(lookup)
	if err := cfg.validate(); err != nil {
		fmt.Fprintf(os.Stderr, "refuse browser fixture target: %v\n", err)
		return 2
	}
	db, err := sql.Open("pgx", cfg.dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open PostgreSQL: %v\n", err)
		return 1
	}
	defer db.Close()
	store, err := objectstore.NewMinIOStore(objectstore.MinIOConfig{
		Endpoint: cfg.endpoint, AccessKey: cfg.accessKey, SecretKey: cfg.secretKey, Bucket: cfg.bucket,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open MinIO: %v\n", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ping PostgreSQL: %v\n", err)
		return 1
	}
	objects := objectstore.NewService(db, store)
	if args[0] == "cleanup" {
		err = cleanup(ctx, db, store)
	} else {
		err = seed(ctx, db, objects, store)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s browser fixture: %v\n", args[0], err)
		return 1
	}
	return 0
}

func loadConfig(lookup func(string) (string, bool)) config {
	get := func(name string) string {
		value, _ := lookup(name)
		return strings.TrimSpace(value)
	}
	return config{
		dsn: get("MOONBOOK_DATABASE_DSN"), endpoint: get("MOONBOOK_MINIO_ENDPOINT"),
		accessKey: get("MINIO_ROOT_USER"), secretKey: get("MINIO_ROOT_PASSWORD"),
		bucket: get("MINIO_BUCKET"), confirmation: get("MOONBOOK_BROWSER_FIXTURE_CONFIRM"),
	}
}

func (c config) validate() error {
	if c.confirmation != fixtureConfirmation {
		return fmt.Errorf("MOONBOOK_BROWSER_FIXTURE_CONFIRM must equal %q", fixtureConfirmation)
	}
	if c.dsn == "" || c.endpoint == "" || c.accessKey == "" || c.secretKey == "" || c.bucket == "" {
		return errors.New("database and MinIO settings are required")
	}
	databaseURL, err := url.Parse(c.dsn)
	if err != nil || databaseURL.Scheme != "postgres" && databaseURL.Scheme != "postgresql" {
		return errors.New("MOONBOOK_DATABASE_DSN must be a PostgreSQL URL")
	}
	if !isLoopback(databaseURL.Hostname()) {
		return errors.New("PostgreSQL host must be loopback")
	}
	endpointURL, err := url.Parse("http://" + strings.TrimPrefix(strings.TrimPrefix(c.endpoint, "http://"), "https://"))
	if err != nil || !isLoopback(endpointURL.Hostname()) {
		return errors.New("MinIO host must be loopback")
	}
	return nil
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func seed(ctx context.Context, db *sql.DB, objects *objectstore.Service, blobs objectstore.BlobStore) (err error) {
	var exists bool
	if err = db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM novel_books WHERE book_name=$1 AND author_name=$2 AND deleted_at IS NULL)`, fixtureBookName, fixtureAuthorName).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return errors.New("fixture already exists; run cleanup first")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var categoryID, authorID, bookID int64
	if err = tx.QueryRowContext(ctx, `INSERT INTO novel_categories(code,name,kind,sort,enabled,source) VALUES($1,'浏览器验收','primary',0,true,'native') RETURNING id`, fixtureCategoryCode).Scan(&categoryID); err != nil {
		return fmt.Errorf("insert category: %w", err)
	}
	if err = tx.QueryRowContext(ctx, `INSERT INTO novel_authors(pen_name,normalized_name,status,source) VALUES($1,$1,'active','native') RETURNING id`, fixtureAuthorName).Scan(&authorID); err != nil {
		return fmt.Errorf("insert author: %w", err)
	}
	if err = tx.QueryRowContext(ctx, `INSERT INTO novel_books(primary_category_id,category_code,category_name,book_name,author_id,author_name,description,score,book_status,publish_status,source_type,featured,featured_sort,featured_note,word_count,charge_mode) VALUES($1,$2,'浏览器验收',$3,$4,$5,'用于验证注册、登录、书架、阅读与历史记录的隔离作品。',9.2,'serializing','published','manual',true,0,'M3 真实浏览器验收',1200,'login_free') RETURNING id`, categoryID, fixtureCategoryCode, fixtureBookName, authorID, fixtureAuthorName).Scan(&bookID); err != nil {
		return fmt.Errorf("insert book: %w", err)
	}
	chapterNames := []string{"第一章 浏览器里的月光", "第二章 被保存的阅读进度"}
	chapterContents := []string{
		"月光落在书页上，这是 M3 浏览器验收的第一章正文。\n读者登录后应当能够完整看到这段内容。\n",
		"翻到第二章时，书架和阅读历史都应保存当前章节。\n再次进入作品，应当可以继续阅读。\n",
	}
	chapterIDs := make([]int64, len(chapterNames))
	for index, name := range chapterNames {
		if err = tx.QueryRowContext(ctx, `INSERT INTO novel_chapters(book_id,chapter_no,chapter_name,word_count,chapter_status,ai_clean_status,source_type) VALUES($1,$2,$3,600,'enabled','pending','manual') RETURNING id`, bookID, index+1, name).Scan(&chapterIDs[index]); err != nil {
			return fmt.Errorf("insert chapter %d: %w", index+1, err)
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE novel_books SET last_chapter_id=$2,last_chapter_name=$3,last_chapter_updated_at=now() WHERE id=$1`, bookID, chapterIDs[1], chapterNames[1]); err != nil {
		return fmt.Errorf("update book chapter: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,status,max_use_count,used_count,remark) VALUES((SELECT COALESCE(max(id),0)+1 FROM reader_invite_codes),$1,'enabled',100,0,'M3 browser fixture')`, fixtureInviteCode); err != nil {
		return fmt.Errorf("insert invite code: %w", err)
	}
	var receivedCallbackID, rejectedCallbackID int64
	if err = tx.QueryRowContext(ctx, `
INSERT INTO reader_payment_callback_logs
    (provider,merchant_order_no,gateway_trade_id,payload_hash,payload_snapshot,signature_valid,processing_result,
     response_status,response_body,request_time,source_type,request_id,trace_id,payload_bytes,payload_truncated)
VALUES ('epusdt','M4-BROWSER-RECEIVED','browser-trade-received',repeat('a',64),
        '{"order_id":"M4-BROWSER-RECEIVED","trade_id":"browser-trade-received","status":"2"}',false,'received',
        0,'',now()-interval '10 minutes','runtime',$1,'m4-browser-trace-received',128,false)
RETURNING id`, fixtureCallbackPrefix+"received").Scan(&receivedCallbackID); err != nil {
		return fmt.Errorf("insert received callback audit: %w", err)
	}
	if err = tx.QueryRowContext(ctx, `
INSERT INTO reader_payment_callback_logs
    (provider,merchant_order_no,gateway_trade_id,payload_hash,payload_snapshot,signature_valid,processing_result,
     failure_code,failure_reason,response_status,response_body,request_time,source_type,request_id,trace_id,payload_bytes,payload_truncated,completed_at)
VALUES ('epusdt','M4-BROWSER-REJECTED','browser-trade-rejected',repeat('b',64),
        '{"order_id":"M4-BROWSER-REJECTED","trade_id":"browser-trade-rejected","amount":"2.00","actual_amount":"1.99","receive_address":"T-browser","token":"usdt","block_transaction_id":"browser-tx","status":"2"}',
        false,'rejected','SIGNATURE_INVALID','Callback signature validation failed',401,'fail',now(),'runtime',$1,'m4-browser-trace-rejected',256,false,now())
RETURNING id`, fixtureCallbackPrefix+"rejected").Scan(&rejectedCallbackID); err != nil {
		return fmt.Errorf("insert rejected callback audit: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = cleanup(context.Background(), db, blobs)
		}
	}()
	for index, chapterID := range chapterIDs {
		object, uploadErr := objects.UploadVerified(ctx, objectstore.Target{Kind: objectstore.KindChapterContent, BookID: bookID, OwnerID: chapterID}, []byte(chapterContents[index]), "text/plain; charset=utf-8")
		if uploadErr != nil {
			return fmt.Errorf("upload chapter %d: %w", index+1, uploadErr)
		}
		if activateErr := objects.Activate(ctx, object.ID); activateErr != nil {
			return fmt.Errorf("activate chapter %d: %w", index+1, activateErr)
		}
	}
	fmt.Printf("fixture_seeded=true book_id=%d chapter_ids=%d,%d invite_code=%s reader_username_prefix=%s callback_ids=%d,%d\n", bookID, chapterIDs[0], chapterIDs[1], fixtureInviteCode, fixtureReaderPrefix, receivedCallbackID, rejectedCallbackID)
	return nil
}

func cleanup(ctx context.Context, db *sql.DB, blobs objectstore.BlobStore) error {
	rows, err := db.QueryContext(ctx, `SELECT object_key FROM novel_objects WHERE book_id IN (SELECT id FROM novel_books WHERE book_name=$1 AND author_name=$2)`, fixtureBookName, fixtureAuthorName)
	if err != nil {
		return err
	}
	var keys []string
	for rows.Next() {
		var key string
		if scanErr := rows.Scan(&key); scanErr != nil {
			rows.Close()
			return scanErr
		}
		keys = append(keys, key)
	}
	if err = rows.Close(); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `ALTER TABLE reader_wallet_ledgers DISABLE TRIGGER reader_wallet_ledgers_immutable_update`); err != nil {
		return fmt.Errorf("disable wallet ledger cleanup trigger: %w", err)
	}
	readerPattern := fixtureReaderPrefix + "%"
	bookArgs := []any{fixtureBookName, fixtureAuthorName}
	statements := append(readerCleanupStatements(readerPattern),
		callbackCleanupStatement(),
		cleanupStatement{`DELETE FROM novel_object_references WHERE book_id IN (SELECT id FROM novel_books WHERE book_name=$1 AND author_name=$2)`, bookArgs},
		cleanupStatement{`DELETE FROM novel_object_events WHERE object_id IN (SELECT id FROM novel_objects WHERE book_id IN (SELECT id FROM novel_books WHERE book_name=$1 AND author_name=$2))`, bookArgs},
		cleanupStatement{`DELETE FROM novel_objects WHERE book_id IN (SELECT id FROM novel_books WHERE book_name=$1 AND author_name=$2)`, bookArgs},
		cleanupStatement{`DELETE FROM novel_chapters WHERE book_id IN (SELECT id FROM novel_books WHERE book_name=$1 AND author_name=$2)`, bookArgs},
		cleanupStatement{`DELETE FROM novel_book_tags WHERE book_id IN (SELECT id FROM novel_books WHERE book_name=$1 AND author_name=$2)`, bookArgs},
		cleanupStatement{`DELETE FROM novel_book_sub_categories WHERE book_id IN (SELECT id FROM novel_books WHERE book_name=$1 AND author_name=$2)`, bookArgs},
		cleanupStatement{`DELETE FROM novel_books WHERE book_name=$1 AND author_name=$2`, bookArgs},
		cleanupStatement{`DELETE FROM novel_authors WHERE pen_name=$1 AND normalized_name=$1`, []any{fixtureAuthorName}},
		cleanupStatement{`DELETE FROM novel_categories WHERE code=$1 AND kind='primary'`, []any{fixtureCategoryCode}},
	)
	for _, statement := range statements {
		if _, err = tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `ALTER TABLE reader_wallet_ledgers ENABLE TRIGGER reader_wallet_ledgers_immutable_update`); err != nil {
		return fmt.Errorf("restore wallet ledger cleanup trigger: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	for _, key := range keys {
		if err = blobs.Remove(ctx, key); err != nil {
			return fmt.Errorf("remove %s: %w", key, err)
		}
	}
	fmt.Printf("fixture_cleaned=true removed_objects=%d reader_username_prefix=%s\n", len(keys), fixtureReaderPrefix)
	return nil
}

func callbackCleanupStatement() cleanupStatement {
	return cleanupStatement{`DELETE FROM reader_payment_callback_logs WHERE request_id LIKE $1`, []any{fixtureCallbackPrefix + "%"}}
}

func readerCleanupStatements(readerPattern string) []cleanupStatement {
	readerIDs := `SELECT id FROM reader_accounts WHERE username LIKE $1`
	return []cleanupStatement{
		{`DELETE FROM reader_payment_callback_logs WHERE recharge_order_id IN (SELECT id FROM reader_recharge_orders WHERE reader_id IN (` + readerIDs + `) OR active_reader_id IN (` + readerIDs + `))`, []any{readerPattern}},
		{`DELETE FROM reader_recharge_orders WHERE reader_id IN (` + readerIDs + `) OR active_reader_id IN (` + readerIDs + `)`, []any{readerPattern}},
		{`DELETE FROM reader_purchase_orders WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM commerce_entitlements WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM commerce_membership_grants WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM reader_checkin_records WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM reader_invite_reward_records WHERE inviter_reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1) OR invitee_reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM reader_wallet_adjustments WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM reader_bonus_coin_buckets WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM reader_daily_activity WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM reader_wallet_ledgers WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM reader_wallets WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM reader_feedback WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM reader_reading_preferences WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM reader_reading_history WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM reader_book_likes WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM reader_bookshelf_entries WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM reader_sessions WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM reader_invite_relations WHERE inviter_reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1) OR invitee_reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM reader_invite_codes WHERE inviter_reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM commerce_reader_search_projection WHERE reader_id IN (SELECT id FROM reader_accounts WHERE username LIKE $1)`, []any{readerPattern}},
		{`DELETE FROM reader_accounts WHERE username LIKE $1`, []any{readerPattern}},
		{`DELETE FROM reader_invite_codes WHERE code=$1`, []any{fixtureInviteCode}},
	}
}
