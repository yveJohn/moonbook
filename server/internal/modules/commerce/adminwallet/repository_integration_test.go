//go:build integration

package adminwallet

import (
	"context"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"testing"
	"time"
)

func TestAdminWalletListAndLedgers(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var readerID int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),2607300000000)+1 FROM reader_accounts`).Scan(&readerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash) VALUES($1,'admin-wallet-fixture','fixture')`, readerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_wallets(reader_id,recharge_coin_balance,total_recharge_coin_income) VALUES($1,88,88)`, readerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_wallet_ledgers(reader_id,ledger_no,biz_type,direction,coin_type,amount,balance_before,balance_after,remark) VALUES($1,'ADMIN-WALLET-FIXTURE','fixture','income','recharge',88,0,88,'fixture')`, readerID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_wallet_ledgers WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_wallets WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})
	r := SQLRepository{DB: db}
	wallets, total, err := r.ListWallets(ctx, "admin-wallet-fixture", 1, 20)
	if err != nil || total != 1 || len(wallets) != 1 || wallets[0].ReaderID == "" || wallets[0].RechargeBalance != "88" {
		t.Fatalf("wallets=%+v total=%d err=%v", wallets, total, err)
	}
	ledgers, total, err := r.ListLedgers(ctx, readerID, "recharge", 1, 20)
	if err != nil || total != 1 || len(ledgers) != 1 || ledgers[0].ID == "" || ledgers[0].BalanceAfter != "88" {
		t.Fatalf("ledgers=%+v total=%d err=%v", ledgers, total, err)
	}
}
