//go:build integration

package checkin

import (
	"context"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
)

func TestCheckinCreditsBonusExactlyOnce(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	readerID := time.Now().UnixNano()
	username := "checkin-it-" + integrationtest.Prefix()
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'x','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_wallet_ledgers WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_checkin_records WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_wallets WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})
	r := SQLRepository{DB: db}
	first, err := r.Checkin(ctx, readerID)
	if err != nil {
		t.Fatal(err)
	}
	if !first.TodayChecked || first.TodayRewardCoin != 10 {
		t.Fatalf("first=%+v", first)
	}
	second, err := r.Checkin(ctx, readerID)
	if err != nil {
		t.Fatal(err)
	}
	if second.TodayRewardCoin != first.TodayRewardCoin {
		t.Fatalf("repeat changed reward: first=%+v second=%+v", first, second)
	}
	var balance, ledgers int64
	if err = db.QueryRowContext(ctx, `SELECT bonus_coin_balance FROM reader_wallets WHERE reader_id=$1`, readerID).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1 AND biz_type='checkin'`, readerID).Scan(&ledgers); err != nil {
		t.Fatal(err)
	}
	if balance != 10 || ledgers != 1 {
		t.Fatalf("balance=%d ledgers=%d", balance, ledgers)
	}
}
