package candidate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/crawlsource"
)

type DiscoveryWorker struct {
	DB           *sql.DB
	WorkerID     string
	PollInterval time.Duration
	Client       *http.Client
	Secrets      crawlsource.SecretResolver
}

func (w *DiscoveryWorker) RunOnce(ctx context.Context) (int, error) {
	if w.DB == nil || strings.TrimSpace(w.WorkerID) == "" {
		return 0, errors.New("candidate discovery worker is not configured")
	}
	rows, err := w.DB.QueryContext(ctx, `SELECT b.id::text
		FROM novel_crawl_forum_board b
		JOIN novel_crawl_forum_source s ON s.id=b.source_id
		WHERE b.enabled AND s.enabled AND b.auto_follow_enabled
		AND (b.last_follow_time IS NULL OR b.last_follow_time <= now() - make_interval(mins => GREATEST(b.follow_interval_minutes,1)))
		ORDER BY COALESCE(b.last_follow_time,'epoch'::timestamptz),b.id
		LIMIT 20`)
	if err != nil {
		return 0, fmt.Errorf("list due discovery boards: %w", err)
	}
	defer rows.Close()
	processed := 0
	var runErr error
	for rows.Next() {
		var boardID string
		if err := rows.Scan(&boardID); err != nil {
			return processed, err
		}
		id, err := strconv.ParseInt(boardID, 10, 64)
		if err != nil || id <= 0 {
			continue
		}
		conn, err := w.DB.Conn(ctx)
		if err != nil {
			return processed, err
		}
		func() {
			defer conn.Close()
			locked, lockErr := w.tryLock(ctx, conn, id)
			if lockErr != nil || !locked {
				return
			}
			repo := SQLRepository{DB: conn}
			target, targetErr := repo.GetBoardTarget(ctx, boardID)
			if targetErr != nil {
				if runErr == nil {
					runErr = targetErr
				}
				return
			}
			if targetErr = resolveTargetCookie(&target, w.Secrets); targetErr != nil {
				if runErr == nil {
					runErr = targetErr
				}
				return
			}
			items, discoverErr := DiscoverBoard(ctx, target, w.client())
			if discoverErr == nil {
				if _, upsertErr := repo.UpsertDiscovered(ctx, target, items); upsertErr == nil {
					_, _ = conn.ExecContext(ctx, `UPDATE novel_crawl_forum_board SET last_follow_time=now(),updated_at=now() WHERE id=$1`, id)
				}
			}
			processed++
		}()
	}
	if err := rows.Err(); err != nil {
		return processed, err
	}
	return processed, runErr
}

func (w *DiscoveryWorker) Run(ctx context.Context) error {
	if w.PollInterval <= 0 {
		w.PollInterval = time.Minute
	}
	if _, err := w.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	ticker := time.NewTicker(w.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := w.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
				continue
			}
		}
	}
}

func (w *DiscoveryWorker) client() *http.Client {
	if w.Client != nil {
		return w.Client
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (w *DiscoveryWorker) tryLock(ctx context.Context, conn *sql.Conn, boardID int64) (bool, error) {
	var locked bool
	err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(hashtextextended($1,0))`, fmt.Sprintf("forum-discovery:%d", boardID)).Scan(&locked)
	return locked, err
}
