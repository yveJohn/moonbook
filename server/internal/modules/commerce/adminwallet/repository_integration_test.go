//go:build integration

package adminwallet

import (
	"context"
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"strconv"
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
	username := fmt.Sprintf("admin-wallet-fixture-%d", readerID)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash) VALUES($1,$2,'fixture')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_wallets(reader_id,recharge_coin_balance,total_recharge_coin_income) VALUES($1,88,88)`, readerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_wallet_ledgers(reader_id,ledger_no,biz_type,direction,coin_type,amount,balance_before,balance_after,remark) VALUES($1,$2,'fixture','income','recharge',88,0,88,'fixture')`, readerID, fmt.Sprintf("ADMIN-WALLET-FIXTURE-%d", readerID)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_wallet_ledgers WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_wallets WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})
	r := SQLRepository{DB: db}
	wallets, total, err := r.ListWallets(ctx, username, 1, 20)
	if err != nil || total != 1 || len(wallets) != 1 || wallets[0].ReaderID == "" || wallets[0].RechargeBalance != "88" {
		t.Fatalf("wallets=%+v total=%d err=%v", wallets, total, err)
	}
	ledgers, total, err := r.ListLedgers(ctx, readerID, "recharge", 1, 20)
	if err != nil || total != 1 || len(ledgers) != 1 || ledgers[0].ID == "" || ledgers[0].BalanceAfter != "88" {
		t.Fatalf("ledgers=%+v total=%d err=%v", ledgers, total, err)
	}
}

func TestAdminWalletAdjustmentIsAtomicAndIdempotent(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var readerID int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),2607300000000)+1 FROM reader_accounts`).Scan(&readerID); err != nil {
		t.Fatal(err)
	}
	username := fmt.Sprintf("admin-adjust-fixture-%d", readerID)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash) VALUES($1,$2,'fixture')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_wallets(reader_id,recharge_coin_balance,total_recharge_coin_income) VALUES($1,100,100)`, readerID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_wallet_ledgers WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_wallets WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})
	r := SQLRepository{DB: db}
	in := AdjustmentInput{ReaderID: formatID(readerID), Amount: "25", CoinType: "recharge", Direction: "income", Reason: "manual correction", RequestID: "adjust-001"}
	first, err := r.Adjust(ctx, in)
	if err != nil || first.Ledger.Amount != "25" || first.Ledger.BalanceBefore != "100" || first.Ledger.BalanceAfter != "125" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	replay, err := r.Adjust(ctx, in)
	if err != nil || replay.Ledger.ID != first.Ledger.ID {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	second, err := r.Adjust(ctx, AdjustmentInput{ReaderID: formatID(readerID), Amount: "10", CoinType: "recharge", Direction: "expense", Reason: "manual correction", RequestID: "adjust-002"})
	if err != nil || second.Ledger.BalanceAfter != "115" {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	if _, err := r.Adjust(ctx, AdjustmentInput{ReaderID: formatID(readerID), Amount: "1000", CoinType: "bonus", Direction: "expense", Reason: "too much", RequestID: "adjust-003"}); err == nil {
		t.Fatal("expected insufficient balance")
	}
}

func formatID(id int64) string { return strconv.FormatInt(id, 10) }
