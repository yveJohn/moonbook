package recharge

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/epusdt"
)

const expiryUpdate = `status='expired',active_reader_id=NULL,
	failure_code=COALESCE(NULLIF(BTRIM(failure_code),''),'ORDER_EXPIRED'),
	failure_message=COALESCE(NULLIF(BTRIM(failure_message),''),'Payment order expired'),updated_at=$1`

type ExpiryRepository interface {
	ExpireBatch(context.Context, time.Time, time.Duration, int) (int64, error)
}

type ExpiryWorker struct {
	Repo                 ExpiryRepository
	BatchSize            int
	UnknownReleaseWindow time.Duration
	LoadWindow           func(context.Context) (time.Duration, error)
	Interval             time.Duration
	Now                  func() time.Time
	OnError              func(error)
}

func (w *ExpiryWorker) RunOnce(ctx context.Context) (int64, error) {
	if err := w.validate(); err != nil {
		return 0, err
	}
	now := time.Now()
	if w.Now != nil {
		now = w.Now()
	}
	window := w.UnknownReleaseWindow
	if w.LoadWindow != nil {
		var err error
		window, err = w.LoadWindow(ctx)
		if err != nil {
			return 0, err
		}
	}
	return w.Repo.ExpireBatch(ctx, now, window, w.BatchSize)
}

func (w *ExpiryWorker) Run(ctx context.Context) error {
	if err := w.validate(); err != nil {
		return err
	}
	if w.Interval <= 0 {
		return errors.New("invalid recharge expiry worker interval")
	}
	scan := func() {
		if _, err := w.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) && w.OnError != nil {
			w.OnError(err)
		}
	}
	scan()
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			scan()
		}
	}
}

func (w *ExpiryWorker) validate() error {
	if w == nil || w.Repo == nil || w.BatchSize <= 0 || (w.UnknownReleaseWindow <= 0 && w.LoadWindow == nil) {
		return errors.New("invalid recharge expiry worker configuration")
	}
	return nil
}

func (r SQLRepository) ExpireBatch(ctx context.Context, now time.Time, window time.Duration, batch int) (int64, error) {
	if r.DB == nil || window <= 0 || batch <= 0 {
		return 0, errors.New("invalid recharge expiry repository configuration")
	}
	result, err := r.DB.ExecContext(ctx, `WITH expired_orders AS (
		SELECT id FROM reader_recharge_orders
		WHERE `+expiryPredicate("$1", "$2")+`
		ORDER BY id
		LIMIT $3
		FOR UPDATE SKIP LOCKED
	)
	UPDATE reader_recharge_orders AS orders SET `+expiryUpdate+`
	FROM expired_orders WHERE orders.id=expired_orders.id`, now, now.Add(-window), batch)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r SQLRepository) expireReader(ctx context.Context, tx *sql.Tx, readerID int64, now time.Time) (int64, error) {
	window, err := r.unknownReleaseWindow(ctx)
	if err != nil {
		return 0, err
	}
	if window <= 0 {
		return 0, errors.New("recharge unknown release window must be positive")
	}
	result, err := tx.ExecContext(ctx, `UPDATE reader_recharge_orders SET `+expiryUpdate+`
		WHERE active_reader_id=$3 AND `+expiryPredicate("$1", "$2"), now, now.Add(-window), readerID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r SQLRepository) unknownReleaseWindow(ctx context.Context) (time.Duration, error) {
	if r.LoadWindow != nil {
		return r.LoadWindow(ctx)
	}
	if r.UnknownReleaseWindow == 0 {
		return epusdt.DefaultUnknownReleaseWindow, nil
	}
	return r.UnknownReleaseWindow, nil
}

func (r SQLRepository) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func expiryPredicate(now, staleBefore string) string {
	return fmt.Sprintf(`(
		(status='pending' AND expire_time IS NOT NULL AND expire_time<=%[1]s)
		OR (status='gateway_unknown' AND ((expire_time IS NOT NULL AND expire_time<=%[1]s) OR (NULLIF(BTRIM(gateway_trade_id),'') IS NULL AND created_at<=%[2]s)))
		OR (status='creating' AND created_at<=%[2]s)
	)`, now, staleBefore)
}
