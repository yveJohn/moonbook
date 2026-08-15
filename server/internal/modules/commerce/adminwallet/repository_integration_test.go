//go:build integration

package adminwallet

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	readercontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/contract"
	readerprovider "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/provider"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
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
	withoutWalletID := readerID + 1
	nickname := fmt.Sprintf("wallet-search-%d", readerID)
	withoutWalletUsername := fmt.Sprintf("admin-wallet-empty-%d", withoutWalletID)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,nickname,password_hash) VALUES($1,$2,$3,'fixture'),($4,$5,$3,'fixture')`, readerID, username, nickname, withoutWalletID, withoutWalletUsername); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_reader_search_projection(reader_id,username,nickname,status) VALUES($1,$2,$3,'enabled'),($4,$5,$3,'enabled')`, readerID, username, nickname, withoutWalletID, withoutWalletUsername); err != nil {
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
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, withoutWalletID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM commerce_reader_search_projection WHERE reader_id=$1`, withoutWalletID)
	})
	r := SQLRepository{DB: db}
	wallets, total, err := r.ListWallets(ctx, username, 1, 20)
	if err != nil || total != 1 || len(wallets) != 1 || wallets[0].ReaderID == "" || wallets[0].RechargeBalance != "88" {
		t.Fatalf("wallets=%+v total=%d err=%v", wallets, total, err)
	}
	firstPage, total, err := r.ListWallets(ctx, nickname, 1, 1)
	if err != nil || total != 2 || len(firstPage) != 1 || firstPage[0].ReaderID != formatID(withoutWalletID) || firstPage[0].RechargeBalance != "0" || firstPage[0].BonusBalance != "0" {
		t.Fatalf("wallet first page=%+v total=%d err=%v", firstPage, total, err)
	}
	secondPage, total, err := r.ListWallets(ctx, nickname, 2, 1)
	if err != nil || total != 2 || len(secondPage) != 1 || secondPage[0].ReaderID != formatID(readerID) || secondPage[0].RechargeBalance != "88" {
		t.Fatalf("wallet second page=%+v total=%d err=%v", secondPage, total, err)
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
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_reader_search_projection(reader_id,username,nickname,status) VALUES($1,$2,'','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_wallets(reader_id,recharge_coin_balance,total_recharge_coin_income) VALUES($1,100,100)`, readerID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `UPDATE reader_accounts SET status='enabled' WHERE id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_wallet_ledgers WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_wallets WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})
	readerAccounts := readerprovider.NewAccount(db)
	service := NewService(SQLRepository{DB: db}, transaction.New(db), readerAccounts)
	in := AdjustmentInput{ReaderID: formatID(readerID), Amount: "25", CoinType: "recharge", Direction: "income", Reason: "manual correction", RequestID: "adjust-001"}
	first, err := service.Adjust(ctx, in)
	if err != nil || first.Ledger.Amount != "25" || first.Ledger.BalanceBefore != "100" || first.Ledger.BalanceAfter != "125" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	replay, err := service.Adjust(ctx, in)
	if err != nil || replay.Ledger.ID != first.Ledger.ID {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	second, err := service.Adjust(ctx, AdjustmentInput{ReaderID: formatID(readerID), Amount: "10", CoinType: "recharge", Direction: "expense", Reason: "manual correction", RequestID: "adjust-002"})
	if err != nil || second.Ledger.BalanceAfter != "115" {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	if _, err := service.Adjust(ctx, AdjustmentInput{ReaderID: formatID(readerID), Amount: "1000", CoinType: "bonus", Direction: "expense", Reason: "too much", RequestID: "adjust-003"}); err == nil {
		t.Fatal("expected insufficient balance")
	}
	if _, err := db.ExecContext(ctx, `UPDATE reader_accounts SET status='disabled' WHERE id=$1`, readerID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Adjust(ctx, AdjustmentInput{ReaderID: formatID(readerID), Amount: "1", CoinType: "recharge", Direction: "income", Reason: "disabled", RequestID: "adjust-disabled"}); !errors.Is(err, readercontract.ErrAccountDisabled) || apperror.Expose(err).Code != apperror.CodeConflict {
		t.Fatalf("disabled adjustment err=%v", err)
	}
	if _, err := service.Adjust(ctx, AdjustmentInput{ReaderID: formatID(readerID + 999), Amount: "1", CoinType: "recharge", Direction: "income", Reason: "missing", RequestID: "adjust-missing"}); !errors.Is(err, readercontract.ErrAccountNotFound) || apperror.Expose(err).Code != apperror.CodeNotFound {
		t.Fatalf("missing adjustment err=%v", err)
	}
	var disabledLedgerCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1 AND idempotency_key=$2`, readerID, "admin_wallet:"+formatID(readerID)+":adjust-disabled").Scan(&disabledLedgerCount); err != nil || disabledLedgerCount != 0 {
		t.Fatalf("disabled adjustment ledger count=%d err=%v", disabledLedgerCount, err)
	}
}

func formatID(id int64) string { return strconv.FormatInt(id, 10) }
