//go:build integration

package invitereward_test

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/adminorder"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/adminrechargeorder"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/payment"
	readerprovider "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/provider"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/transaction"
)

func TestFirstRechargeRewardIsIdempotentAcrossRechargeEntries(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	base := time.Now().UnixNano()
	inviterID, inviteeID, inviteCodeID, rechargeOrderID, paymentOrderID := base, base+1, base+2, base+3, base+4
	suffix := strconv.FormatInt(base, 10)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash) VALUES($1,$2,'fixture'),($3,$4,'fixture')`, inviterID, "cross-inviter-"+suffix, inviteeID, "cross-invitee-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_reader_search_projection(reader_id,username,nickname,status) VALUES($1,$2,'','enabled'),($3,$4,'','enabled')`, inviterID, "cross-inviter-"+suffix, inviteeID, "cross-invitee-"+suffix); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status) VALUES($1,$2,$3,'enabled')`, inviteCodeID, "CROSS-"+suffix, inviterID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_relations(inviter_reader_id,invitee_reader_id,invite_code_id,status) VALUES($1,$2,$3,'active')`, inviterID, inviteeID, inviteCodeID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_recharge_orders(id,order_no,reader_id,request_id,source_type,diamond_amount,price_usdt,provider,currency,token,network,status) VALUES($1,$2,$3,$4,'custom',100,'1.00','epusdt','usd','usdt','tron','gateway_unknown')`, rechargeOrderID, "CROSS-MANUAL-"+suffix, inviteeID, "cross-manual-"+suffix); err != nil {
		t.Fatal(err)
	}
	paymentOrderNo := "CROSS-PAYMENT-" + suffix
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_recharge_orders(id,order_no,reader_id,request_id,source_type,diamond_amount,price_usdt,provider,currency,token,network,status) VALUES($1,$2,$3,$4,'custom',40,'2.00','epusdt','usd','usdt','tron','pending')`, paymentOrderID, paymentOrderNo, inviteeID, "cross-payment-"+suffix); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		q := context.Background()
		_, _ = db.ExecContext(q, `DELETE FROM reader_payment_callback_logs WHERE recharge_order_id IN (SELECT id FROM reader_recharge_orders WHERE reader_id=$1)`, inviteeID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_invite_reward_records WHERE invitee_reader_id=$1`, inviteeID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_wallet_ledgers WHERE reader_id IN ($1,$2)`, inviterID, inviteeID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_wallets WHERE reader_id IN ($1,$2)`, inviterID, inviteeID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_purchase_orders WHERE reader_id=$1`, inviteeID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_recharge_orders WHERE reader_id=$1`, inviteeID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_invite_relations WHERE invitee_reader_id=$1`, inviteeID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_invite_codes WHERE id=$1`, inviteCodeID)
		_, _ = db.ExecContext(q, `DELETE FROM commerce_reader_search_projection WHERE reader_id IN ($1,$2)`, inviterID, inviteeID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_accounts WHERE id IN ($1,$2)`, inviterID, inviteeID)
	})

	transactor := transaction.New(db)
	readerAccounts := readerprovider.NewAccount(db)
	readerInvites := readerprovider.NewInvite(db)
	manualService := adminrechargeorder.NewService(adminrechargeorder.SQLRepository{DB: db, Invites: readerInvites, Accounts: readerAccounts}, transactor, readerAccounts)
	mockService := adminorder.NewService(adminorder.SQLRepository{DB: db, Invites: readerInvites}, transactor, readerAccounts)
	mockOrder, err := mockService.CreateMockRecharge(ctx, adminorder.MockRechargeInput{
		ReaderID:           fmt.Sprint(inviteeID),
		RechargeCoinAmount: "25",
		RequestID:          "cross-mock-request-" + suffix,
		Remark:             "跨入口首充奖励测试",
	}, 501)
	if err != nil {
		t.Fatal(err)
	}
	mockOrderID, err := strconv.ParseInt(mockOrder.ID, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	callback := payment.Callback{TradeID: "cross-payment-trade-" + suffix, OrderNo: paymentOrderNo, Amount: "2.00", ActualAmount: "2.00", ReceiveAddress: "T-cross", Token: "usdt", TransactionID: "cross-payment-tx-" + suffix, Status: 2, Fields: map[string]string{"order_id": paymentOrderNo}}
	paymentRepository := payment.SQLRepository{DB: db, Invites: readerInvites, Accounts: readerAccounts}
	start := make(chan struct{})
	errs := make(chan error, 3)
	var workers sync.WaitGroup
	workers.Add(3)
	go func() {
		defer workers.Done()
		<-start
		_, err := manualService.ManualPay(ctx, rechargeOrderID, adminrechargeorder.ManualPayInput{RequestID: "cross-manual-request-" + suffix, GatewayTradeID: "cross-manual-trade-" + suffix, ActualAmount: "1.00", Remark: "跨入口首充奖励测试"})
		if err != nil {
			err = fmt.Errorf("manual pay: %w", err)
		}
		errs <- err
	}()
	go func() {
		defer workers.Done()
		<-start
		_, err := mockService.ConfirmMockRecharge(ctx, mockOrderID, 501)
		if err != nil {
			err = fmt.Errorf("mock recharge: %w", err)
		}
		errs <- err
	}()
	go func() {
		defer workers.Done()
		<-start
		err := paymentRepository.Process(ctx, callback)
		if err != nil {
			err = fmt.Errorf("payment callback: %w", err)
		}
		errs <- err
	}()
	close(start)
	workers.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	var rewardRows, rewardLedgers int
	var inviterBonus, inviteeRecharge int64
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM reader_invite_reward_records WHERE invitee_reader_id=$1 AND reward_stage='first_recharge'`, inviteeID).Scan(&rewardRows); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1 AND biz_type='invite_first_recharge_reward'`, inviterID).Scan(&rewardLedgers); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT bonus_coin_balance FROM reader_wallets WHERE reader_id=$1`, inviterID).Scan(&inviterBonus); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, `SELECT recharge_coin_balance FROM reader_wallets WHERE reader_id=$1`, inviteeID).Scan(&inviteeRecharge); err != nil {
		t.Fatal(err)
	}
	if rewardRows != 1 || rewardLedgers != 1 || inviterBonus != 100 || inviteeRecharge != 165 {
		t.Fatalf("rewardRows=%d rewardLedgers=%d inviterBonus=%d inviteeRecharge=%d", rewardRows, rewardLedgers, inviterBonus, inviteeRecharge)
	}
}
