package legacymigrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/objectstore"
)

type NovelBookCoversStage struct {
	Objects    *objectstore.Service
	Downloader *CoverDownloader
}

func (NovelBookCoversStage) Name() string { return "novel-book-covers" }

func (stage NovelBookCoversStage) RunBatch(ctx context.Context, _ *sql.DB, target *sql.Tx, cursor string, limit int) (BatchResult, error) {
	if stage.Objects == nil || stage.Downloader == nil {
		return BatchResult{}, errors.New("cover migration requires object storage and downloader")
	}
	lastID, err := parseCursor(cursor)
	if err != nil {
		return BatchResult{}, err
	}
	rows, err := target.QueryContext(ctx, `SELECT id,legacy_cover_url FROM novel_books
		WHERE id>$1 AND deleted_at IS NULL AND btrim(legacy_cover_url)<>'' ORDER BY id LIMIT $2`, lastID, limit)
	if err != nil {
		return BatchResult{}, fmt.Errorf("list legacy book covers: %w", err)
	}
	type sourceCover struct {
		bookID int64
		url    string
	}
	covers := make([]sourceCover, 0, limit)
	for rows.Next() {
		var cover sourceCover
		if err := rows.Scan(&cover.bookID, &cover.url); err != nil {
			rows.Close()
			return BatchResult{}, err
		}
		covers = append(covers, cover)
	}
	if err := rows.Close(); err != nil {
		return BatchResult{}, err
	}
	result := BatchResult{NextCursor: cursor, Metadata: map[string]any{"source": "novel_books.legacy_cover_url"}}
	for _, cover := range covers {
		result.Processed++
		result.NextCursor = strconv.FormatInt(cover.bookID, 10)
		recordError := func(code, message string, retryable bool) {
			result.Errors = append(result.Errors, RecordError{SourceTable: "novel_book", SourceID: result.NextCursor, Code: code, Message: message, Retryable: retryable})
		}
		data, err := stage.Downloader.Download(ctx, cover.url)
		if err != nil {
			recordError("COVER_DOWNLOAD_FAILED", "legacy cover download failed; inspect the source row and network policy", true)
			continue
		}
		contentType, extension, ok := objectstore.DetectCover(data)
		if !ok {
			recordError("INVALID_COVER_TYPE", "legacy cover is not JPEG, PNG, WebP, or GIF", false)
			continue
		}
		digest := sha256.Sum256([]byte(strings.TrimSpace(cover.url)))
		fingerprint := hex.EncodeToString(digest[:])
		object, err := stage.Objects.UploadVerifiedWithOptions(ctx,
			objectstore.Target{Kind: objectstore.KindBookCover, BookID: cover.bookID, OwnerID: cover.bookID, Extension: extension},
			data, contentType, objectstore.UploadOptions{Source: "legacy", SourceFingerprint: fingerprint})
		if err != nil {
			recordError("COVER_UPLOAD_FAILED", "legacy cover object upload failed", true)
			continue
		}
		if object.State == objectstore.StateVerified {
			if err := stage.Objects.Activate(ctx, object.ID); err != nil {
				recordError("COVER_ACTIVATE_FAILED", "legacy cover reference activation failed", true)
				continue
			}
		}
	}
	result.Done = result.Processed < int64(limit)
	return result, nil
}
