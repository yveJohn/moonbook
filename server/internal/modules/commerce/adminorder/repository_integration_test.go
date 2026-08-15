//go:build integration

package adminorder

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

func TestListAndGetPurchaseOrders(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var readerID, orderID, secondOrderID int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),2607300010000)+1 FROM reader_accounts`).Scan(&readerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash) VALUES($1,$2,'fixture')`, readerID, "admin-order-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_reader_search_projection(reader_id,username,nickname,status) VALUES($1,$2,'','enabled')`, readerID, "admin-order-"+suffix); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_purchase_orders(order_no,reader_id,order_type,product_type,product_name_snapshot,price_coin_snapshot,recharge_coin_amount,bonus_coin_amount,status,idempotency_key,paid_time) VALUES($1,$2,'membership','membership','集成会员',60,40,20,'paid',$3,now()) RETURNING id`, "ADMIN-PURCHASE-"+suffix, readerID, "admin-purchase-"+suffix).Scan(&orderID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_purchase_orders(order_no,reader_id,order_type,product_type,product_name_snapshot,price_coin_snapshot,recharge_coin_amount,bonus_coin_amount,status,idempotency_key,paid_time,created_at) VALUES($1,$2,'membership','membership','集成会员二',30,30,0,'paid',$3,now(),'2026-01-01 00:00:00+00') RETURNING id`, "ADMIN-PURCHASE-SECOND-"+suffix, readerID, "admin-purchase-second-"+suffix).Scan(&secondOrderID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE reader_purchase_orders SET created_at='2026-01-01 00:00:00+00' WHERE id=$1`, orderID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_purchase_orders WHERE id IN ($1,$2)`, orderID, secondOrderID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM commerce_reader_search_projection WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})
	r := SQLRepository{DB: db}
	items, total, err := r.List(ctx, "ADMIN-PURCHASE-"+suffix, "membership", "paid", 1, 20)
	if err != nil || total != 1 || len(items) != 1 || items[0].ID == "" || items[0].ReaderID == "" || items[0].PriceCoin != "60" {
		t.Fatalf("items=%+v total=%d err=%v", items, total, err)
	}
	got, err := r.Get(ctx, orderID)
	if err != nil || got.ID == "" || got.ReaderUsername != "admin-order-"+suffix || got.BonusCoinAmount != "20" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	username := "admin-order-" + suffix
	firstPage, total, err := r.List(ctx, username, "membership", "paid", 1, 1)
	if err != nil || total != 2 || len(firstPage) != 1 || firstPage[0].ID != strconv.FormatInt(secondOrderID, 10) {
		t.Fatalf("first page=%+v total=%d err=%v", firstPage, total, err)
	}
	secondPage, total, err := r.List(ctx, username, "membership", "paid", 2, 1)
	if err != nil || total != 2 || len(secondPage) != 1 || secondPage[0].ID != strconv.FormatInt(orderID, 10) {
		t.Fatalf("second page=%+v total=%d err=%v", secondPage, total, err)
	}
}

func TestMockRechargeCreateConfirmAndFirstInviteReward(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var inviterID, inviteeID, inviteCodeID int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(id),2607300010000)+1 FROM reader_accounts`).Scan(&inviterID); err != nil {
		t.Fatal(err)
	}
	inviteeID = inviterID + 1
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash) VALUES($1,$2,'fixture'),($3,$4,'fixture')`, inviterID, "mock-inviter-"+suffix, inviteeID, "mock-invitee-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_reader_search_projection(reader_id,username,nickname,status) VALUES($1,$2,'','enabled'),($3,$4,'','enabled')`, inviterID, "mock-inviter-"+suffix, inviteeID, "mock-invitee-"+suffix); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status) VALUES((SELECT COALESCE(max(id),0)+1 FROM reader_invite_codes),$1,$2,'enabled') RETURNING id`, "MOCK-"+suffix, inviterID).Scan(&inviteCodeID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_relations(inviter_reader_id,invitee_reader_id,invite_code_id,status) VALUES($1,$2,$3,'active')`, inviterID, inviteeID, inviteCodeID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `UPDATE reader_accounts SET status='enabled' WHERE id=$1`, inviteeID)
	})

	readerAccounts := readerprovider.NewAccount(db)
	service := NewService(SQLRepository{DB: db}, transaction.New(db), readerAccounts)
	in := MockRechargeInput{ReaderID: fmt.Sprint(inviteeID), RechargeCoinAmount: "200", RequestID: "request-" + suffix, Remark: "集成模拟充值"}
	created, err := service.CreateMockRecharge(ctx, in, 501)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := service.CreateMockRecharge(ctx, in, 501)
	if err != nil || duplicate.ID != created.ID || duplicate.Status != "pending" {
		t.Fatalf("duplicate=%+v created=%+v err=%v", duplicate, created, err)
	}
	orderID, err := strconv.ParseInt(created.ID, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	paid, err := service.ConfirmMockRecharge(ctx, orderID, 501)
	if err != nil || paid.Status != "paid" {
		t.Fatalf("paid=%+v err=%v", paid, err)
	}
	paidAgain, err := service.ConfirmMockRecharge(ctx, orderID, 502)
	if err != nil || paidAgain.Status != "paid" {
		t.Fatalf("paidAgain=%+v err=%v", paidAgain, err)
	}

	var inviteeRecharge, inviterBonus, orderCount, rechargeLedgers, rewardRows, rewardLedgers int
	if err = db.QueryRowContext(ctx, `SELECT recharge_coin_balance FROM reader_wallets WHERE reader_id=$1`, inviteeID).Scan(&inviteeRecharge); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT bonus_coin_balance FROM reader_wallets WHERE reader_id=$1`, inviterID).Scan(&inviterBonus); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM reader_purchase_orders WHERE reader_id=$1 AND idempotency_key=$2`, inviteeID, "mock_recharge_create:"+in.RequestID).Scan(&orderCount); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1 AND biz_type='mock_recharge'`, inviteeID).Scan(&rechargeLedgers); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM reader_invite_reward_records WHERE invitee_reader_id=$1 AND reward_stage='first_recharge'`, inviteeID).Scan(&rewardRows); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1 AND biz_type='invite_first_recharge_reward'`, inviterID).Scan(&rewardLedgers); err != nil {
		t.Fatal(err)
	}
	if inviteeRecharge != 200 || inviterBonus != 100 || orderCount != 1 || rechargeLedgers != 1 || rewardRows != 1 || rewardLedgers != 1 {
		t.Fatalf("invitee=%d inviter=%d orders=%d rechargeLedgers=%d rewards=%d rewardLedgers=%d", inviteeRecharge, inviterBonus, orderCount, rechargeLedgers, rewardRows, rewardLedgers)
	}
	if _, err := db.ExecContext(ctx, `UPDATE reader_accounts SET status='disabled' WHERE id=$1`, inviteeID); err != nil {
		t.Fatal(err)
	}
	disabledInput := in
	disabledInput.RequestID = "disabled-" + suffix
	if _, err := service.CreateMockRecharge(ctx, disabledInput, 501); !errors.Is(err, readercontract.ErrAccountDisabled) || apperror.Expose(err).Code != apperror.CodeConflict {
		t.Fatalf("disabled mock recharge err=%v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_purchase_orders WHERE reader_id=$1 AND idempotency_key=$2`, inviteeID, "mock_recharge_create:"+disabledInput.RequestID).Scan(&orderCount); err != nil || orderCount != 0 {
		t.Fatalf("disabled mock recharge order count=%d err=%v", orderCount, err)
	}
	missingInput := disabledInput
	missingInput.ReaderID = fmt.Sprint(inviteeID + 999)
	missingInput.RequestID = "missing-" + suffix
	if _, err := service.CreateMockRecharge(ctx, missingInput, 501); !errors.Is(err, readercontract.ErrAccountNotFound) || apperror.Expose(err).Code != apperror.CodeNotFound {
		t.Fatalf("missing mock recharge err=%v", err)
	}
}
