//go:build integration

package payment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/epusdt"
	readerprovider "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/provider"
)

type lateCallbackFixture struct {
	db        *sql.DB
	ctx       context.Context
	readerID  int64
	inviterID int64
	orderID   int64
	orderNo   string
	tradeID   string
	txID      string
}

func newLateCallbackFixture(t *testing.T, status string, offset int64) lateCallbackFixture {
	t.Helper()
	db, _ := integrationtest.RequireDB(t)
	ctx := context.Background()
	base := time.Now().UnixNano() + offset*10
	fixture := lateCallbackFixture{
		db: db, ctx: ctx, inviterID: base, readerID: base + 1, orderID: base + 3,
		orderNo: fmt.Sprintf("LATE-%d-%s", base, integrationtest.Prefix()),
		tradeID: fmt.Sprintf("late-trade-%d", base), txID: fmt.Sprintf("late-tx-%d", base),
	}
	codeID := base + 2
	username := fmt.Sprintf("late-callback-%d", base)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'x','enabled'),($3,$4,'x','enabled')`, fixture.inviterID, username+"-inviter", fixture.readerID, username+"-reader"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status) VALUES($1,$2,$3,'enabled')`, codeID, username, fixture.inviterID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_relations(inviter_reader_id,invitee_reader_id,invite_code_id,status) VALUES($1,$2,$3,'active')`, fixture.inviterID, fixture.readerID, codeID); err != nil {
		t.Fatal(err)
	}
	var active any
	if status == "pending" || status == "gateway_unknown" {
		active = fixture.readerID
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_recharge_orders(id,order_no,reader_id,request_id,source_type,diamond_amount,price_usdt,provider,currency,token,network,gateway_trade_id,actual_amount,receive_address,status,credential_ref,merchant_pid_snapshot,active_reader_id) VALUES($1,$2,$3,$4,'custom',17,2.00,'epusdt','usd','usdt','tron',$5,2.00,'T-address',$6,'primary','merchant',$7)`, fixture.orderID, fixture.orderNo, fixture.readerID, "request-"+fixture.orderNo, fixture.tradeID, status, active); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		q := context.Background()
		_, _ = db.ExecContext(q, `DELETE FROM reader_payment_callback_logs WHERE recharge_order_id=$1`, fixture.orderID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_wallet_ledgers WHERE reader_id IN ($1,$2)`, fixture.readerID, fixture.inviterID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_invite_reward_records WHERE invitee_reader_id=$1`, fixture.readerID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_wallets WHERE reader_id IN ($1,$2)`, fixture.readerID, fixture.inviterID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_recharge_orders WHERE id=$1`, fixture.orderID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_invite_relations WHERE invitee_reader_id=$1`, fixture.readerID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_invite_codes WHERE id=$1`, codeID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_accounts WHERE id IN ($1,$2)`, fixture.readerID, fixture.inviterID)
	})
	return fixture
}

func (fixture lateCallbackFixture) repository() SQLRepository {
	return SQLRepository{DB: fixture.db, Invites: readerprovider.NewInvite(fixture.db), Accounts: readerprovider.NewAccount(fixture.db)}
}

func (fixture lateCallbackFixture) callback() Callback {
	return Callback{PID: "merchant", TradeID: fixture.tradeID, OrderNo: fixture.orderNo, Amount: "2.00", ActualAmount: "2.00", ReceiveAddress: "T-address", Token: "usdt", TransactionID: fixture.txID, Status: 2, Fields: map[string]string{"order_id": fixture.orderNo, "trade_id": fixture.tradeID}}
}

func (fixture lateCallbackFixture) signedCallback(pid, secret string) Callback {
	fields := map[string]string{
		"pid": pid, "trade_id": fixture.tradeID, "order_id": fixture.orderNo, "amount": "2.00", "actual_amount": "2.00",
		"receive_address": "T-address", "token": "usdt", "block_transaction_id": fixture.txID, "status": "2",
	}
	fields["signature"] = Sign(fields, secret)
	return Callback{
		PID: pid, TradeID: fixture.tradeID, OrderNo: fixture.orderNo, Amount: "2.00", ActualAmount: "2.00",
		ReceiveAddress: "T-address", Token: "usdt", TransactionID: fixture.txID, Signature: fields["signature"], Status: 2, Fields: fields,
	}
}

func TestCallbackServiceUsesPersistedCredentialSnapshot(t *testing.T) {
	credentials, err := epusdt.NewCredentialProvider("primary", "merchant-current", "current-secret", `[{"ref":"previous","pid":"merchant-previous","secret":"previous-secret"}]`)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name          string
		credentialRef any
		merchantPID   any
		callbackPID   string
		secret        string
		wantReject    bool
	}{
		{name: "current", credentialRef: "primary", merchantPID: "merchant-current", callbackPID: "merchant-current", secret: "current-secret"},
		{name: "historical", credentialRef: "previous", merchantPID: "merchant-previous", callbackPID: "merchant-previous", secret: "previous-secret"},
		{name: "legacy", credentialRef: nil, merchantPID: nil, callbackPID: "merchant-current", secret: "current-secret"},
		{name: "unknown", credentialRef: "missing", merchantPID: "merchant-current", callbackPID: "merchant-current", secret: "current-secret", wantReject: true},
		{name: "pid mismatch", credentialRef: "previous", merchantPID: "merchant-current", callbackPID: "merchant-current", secret: "previous-secret", wantReject: true},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newLateCallbackFixture(t, "pending", int64(index+60))
			if _, err := fixture.db.Exec(`UPDATE reader_recharge_orders SET credential_ref=$1,merchant_pid_snapshot=$2 WHERE id=$3`, test.credentialRef, test.merchantPID, fixture.orderID); err != nil {
				t.Fatal(err)
			}
			repository := fixture.repository()
			err := NewService(repository, credentials).Process(fixture.ctx, fixture.signedCallback(test.callbackPID, test.secret))
			if test.wantReject {
				if !errors.Is(err, ErrUnauthorized) {
					t.Fatalf("err=%v", err)
				}
				var ledgers int
				if err := fixture.db.QueryRow(`SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1`, fixture.readerID).Scan(&ledgers); err != nil || ledgers != 0 {
					t.Fatalf("ledgers=%d err=%v", ledgers, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var status string
			if err := fixture.db.QueryRow(`SELECT status FROM reader_recharge_orders WHERE id=$1`, fixture.orderID).Scan(&status); err != nil || status != "paid" {
				t.Fatalf("status=%s err=%v", status, err)
			}
		})
	}
}

func TestLateCallbackAllowedStatesCreditExactlyOnce(t *testing.T) {
	for index, status := range []string{"pending", "gateway_unknown", "superseded", "expired", "callback_exception"} {
		t.Run(status, func(t *testing.T) {
			fixture := newLateCallbackFixture(t, status, int64(index+1))
			callback := fixture.callback()
			if err := fixture.repository().Process(fixture.ctx, callback); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repository().Process(fixture.ctx, callback); err != nil {
				t.Fatal(err)
			}
			var gotStatus string
			var active sql.NullInt64
			var readerBalance, inviterBalance int64
			var rechargeLedgers, rewards, rewardLedgers, callbacks int
			if err := fixture.db.QueryRow(`SELECT status,active_reader_id FROM reader_recharge_orders WHERE id=$1`, fixture.orderID).Scan(&gotStatus, &active); err != nil {
				t.Fatal(err)
			}
			if err := fixture.db.QueryRow(`SELECT recharge_coin_balance FROM reader_wallets WHERE reader_id=$1`, fixture.readerID).Scan(&readerBalance); err != nil {
				t.Fatal(err)
			}
			if err := fixture.db.QueryRow(`SELECT bonus_coin_balance FROM reader_wallets WHERE reader_id=$1`, fixture.inviterID).Scan(&inviterBalance); err != nil {
				t.Fatal(err)
			}
			if err := fixture.db.QueryRow(`SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1 AND biz_type='epusdt_recharge'`, fixture.readerID).Scan(&rechargeLedgers); err != nil {
				t.Fatal(err)
			}
			if err := fixture.db.QueryRow(`SELECT count(*) FROM reader_invite_reward_records WHERE invitee_reader_id=$1 AND reward_stage='first_recharge'`, fixture.readerID).Scan(&rewards); err != nil {
				t.Fatal(err)
			}
			if err := fixture.db.QueryRow(`SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1 AND biz_type='invite_first_recharge_reward'`, fixture.inviterID).Scan(&rewardLedgers); err != nil {
				t.Fatal(err)
			}
			if err := fixture.db.QueryRow(`SELECT count(*) FROM reader_payment_callback_logs WHERE recharge_order_id=$1`, fixture.orderID).Scan(&callbacks); err != nil {
				t.Fatal(err)
			}
			if gotStatus != "paid" || active.Valid || readerBalance != 17 || inviterBalance != 100 || rechargeLedgers != 1 || rewards != 1 || rewardLedgers != 1 || callbacks != 1 {
				t.Fatalf("status=%s active=%+v balances=%d/%d ledgers=%d rewards=%d/%d callbacks=%d", gotStatus, active, readerBalance, inviterBalance, rechargeLedgers, rewards, rewardLedgers, callbacks)
			}
		})
	}
}

func TestLateCallbackRejectsDisallowedStatesWithoutFinancialFacts(t *testing.T) {
	for index, status := range []string{"creating", "create_failed"} {
		t.Run(status, func(t *testing.T) {
			fixture := newLateCallbackFixture(t, status, int64(index+20))
			if err := fixture.repository().Process(fixture.ctx, fixture.callback()); !errors.Is(err, ErrRejected) {
				t.Fatalf("err=%v", err)
			}
			var ledgers, rewards, callbacks int
			if err := fixture.db.QueryRow(`SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id IN ($1,$2)`, fixture.readerID, fixture.inviterID).Scan(&ledgers); err != nil {
				t.Fatal(err)
			}
			if err := fixture.db.QueryRow(`SELECT count(*) FROM reader_invite_reward_records WHERE invitee_reader_id=$1`, fixture.readerID).Scan(&rewards); err != nil {
				t.Fatal(err)
			}
			if err := fixture.db.QueryRow(`SELECT count(*) FROM reader_payment_callback_logs WHERE recharge_order_id=$1`, fixture.orderID).Scan(&callbacks); err != nil {
				t.Fatal(err)
			}
			if ledgers != 0 || rewards != 0 || callbacks != 0 {
				t.Fatalf("ledgers=%d rewards=%d callbacks=%d", ledgers, rewards, callbacks)
			}
		})
	}
}

func TestLateCallbackRejectsGatewayTradeAndTransactionReplay(t *testing.T) {
	first := newLateCallbackFixture(t, "pending", 40)
	second := newLateCallbackFixture(t, "pending", 41)
	third := newLateCallbackFixture(t, "pending", 42)
	if err := first.repository().Process(first.ctx, first.callback()); err != nil {
		t.Fatal(err)
	}

	tradeReplay := second.callback()
	tradeReplay.TradeID = first.tradeID
	if err := second.repository().Process(second.ctx, tradeReplay); !errors.Is(err, ErrRejected) {
		t.Fatalf("trade replay err=%v", err)
	}
	transactionReplay := third.callback()
	transactionReplay.TransactionID = first.txID
	if err := third.repository().Process(third.ctx, transactionReplay); !errors.Is(err, ErrRejected) {
		t.Fatalf("transaction replay err=%v", err)
	}
	differentPaidTrade := first.callback()
	differentPaidTrade.TradeID = "different-trade"
	if err := first.repository().Process(first.ctx, differentPaidTrade); !errors.Is(err, ErrRejected) {
		t.Fatalf("paid trade mismatch err=%v", err)
	}
	differentPaidTransaction := first.callback()
	differentPaidTransaction.TransactionID = "different-transaction"
	if err := first.repository().Process(first.ctx, differentPaidTransaction); !errors.Is(err, ErrRejected) {
		t.Fatalf("paid transaction mismatch err=%v", err)
	}
	differentPaidAmount := first.callback()
	differentPaidAmount.Amount = "3.00"
	if err := first.repository().Process(first.ctx, differentPaidAmount); !errors.Is(err, ErrRejected) {
		t.Fatalf("paid amount mismatch err=%v", err)
	}
}
