//go:build integration

package payment

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/integrationtest"
	readercontract "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/contract"
	readerprovider "github.com/flipped-aurora/gin-vue-admin/server/internal/modules/reader/provider"
)

type failingInviteReader struct{ err error }

func (reader failingInviteReader) ActiveInviteRelation(context.Context, int64) (readercontract.InviteRelation, error) {
	return readercontract.InviteRelation{}, reader.err
}

func TestCallbackCreditsRechargeExactlyOnce(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	readerID, orderID := base, base+1
	username := "payment-it-" + integrationtest.Prefix()
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'x','enabled')`, readerID, username); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_wallets(reader_id) VALUES($1)`, readerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE reader_payment_channels SET enabled=true WHERE provider='epusdt'`); err != nil {
		t.Fatal(err)
	}
	orderNo := "PAY" + integrationtest.Prefix()
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_recharge_orders(id,order_no,reader_id,request_id,source_type,diamond_amount,price_usdt,provider,currency,token,network,status) VALUES($1,$2,$3,$4,'custom',17,2.00,'epusdt','usd','usdt','tron','pending')`, orderID, orderNo, readerID, "req-"+integrationtest.Prefix()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		q := context.Background()
		_, _ = db.ExecContext(q, `DELETE FROM reader_payment_callback_logs WHERE recharge_order_id=$1`, orderID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_wallet_ledgers WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_recharge_orders WHERE id=$1`, orderID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_wallets WHERE reader_id=$1`, readerID)
		_, _ = db.ExecContext(q, `UPDATE reader_payment_channels SET enabled=false WHERE provider='epusdt'`)
		_, _ = db.ExecContext(q, `DELETE FROM reader_accounts WHERE id=$1`, readerID)
	})
	fields := map[string]string{"pid": "merchant", "trade_id": "trade-1", "order_id": orderNo, "amount": "2.00", "actual_amount": "2.00", "receive_address": "Taddress", "token": "USDT", "block_transaction_id": "tx-1", "status": "2"}
	fields["signature"] = Sign(fields, "secret")
	callback := Callback{PID: fields["pid"], TradeID: fields["trade_id"], OrderNo: orderNo, Amount: fields["amount"], ActualAmount: fields["actual_amount"], ReceiveAddress: fields["receive_address"], Token: fields["token"], TransactionID: fields["block_transaction_id"], Status: 2, Fields: fields}
	r := SQLRepository{DB: db, Invites: readerprovider.NewInvite(db)}
	if err := r.Process(ctx, callback); err != nil {
		t.Fatal(err)
	}
	if err := r.Process(ctx, callback); err != nil {
		t.Fatal(err)
	}
	var balance, ledgers int64
	if err := db.QueryRowContext(ctx, `SELECT recharge_coin_balance FROM reader_wallets WHERE reader_id=$1`, readerID).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1`, readerID).Scan(&ledgers); err != nil {
		t.Fatal(err)
	}
	if balance != 17 || ledgers != 1 {
		t.Fatalf("balance=%d ledgers=%d", balance, ledgers)
	}
}

func TestCallbackRollsBackWhenInviteDependencyOrRewardWalletFails(t *testing.T) {
	db, _ := integrationtest.RequireDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	base := time.Now().UnixNano()
	inviterID, inviteeID, codeID, orderID := base, base+1, base+2, base+3
	suffix := fmt.Sprintf("payment-rollback-%d", base)
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_accounts(id,username,password_hash,status) VALUES($1,$2,'x','enabled'),($3,$4,'x','enabled')`, inviterID, suffix+"-inviter", inviteeID, suffix+"-invitee"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO commerce_reader_search_projection(reader_id,username,nickname,status) VALUES($1,$2,'','enabled'),($3,$4,'','enabled')`, inviterID, suffix+"-inviter", inviteeID, suffix+"-invitee"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_codes(id,code,inviter_reader_id,status) VALUES($1,$2,$3,'enabled')`, codeID, suffix, inviterID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_invite_relations(inviter_reader_id,invitee_reader_id,invite_code_id,status) VALUES($1,$2,$3,'active')`, inviterID, inviteeID, codeID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_wallets(reader_id,bonus_coin_balance,total_bonus_coin_income) VALUES($1,9223372036854775800,9223372036854775800)`, inviterID); err != nil {
		t.Fatal(err)
	}
	orderNo := "PAY-ROLLBACK-" + suffix
	if _, err := db.ExecContext(ctx, `INSERT INTO reader_recharge_orders(id,order_no,reader_id,request_id,source_type,diamond_amount,price_usdt,provider,currency,token,network,status) VALUES($1,$2,$3,$4,'custom',17,2.00,'epusdt','usd','usdt','tron','pending')`, orderID, orderNo, inviteeID, "req-"+suffix); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		q := context.Background()
		_, _ = db.ExecContext(q, `DELETE FROM reader_payment_callback_logs WHERE recharge_order_id=$1`, orderID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_recharge_orders WHERE id=$1`, orderID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_wallets WHERE reader_id IN ($1,$2)`, inviterID, inviteeID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_invite_reward_records WHERE invitee_reader_id=$1`, inviteeID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_invite_relations WHERE invitee_reader_id=$1`, inviteeID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_invite_codes WHERE id=$1`, codeID)
		_, _ = db.ExecContext(q, `DELETE FROM commerce_reader_search_projection WHERE reader_id IN ($1,$2)`, inviterID, inviteeID)
		_, _ = db.ExecContext(q, `DELETE FROM reader_accounts WHERE id IN ($1,$2)`, inviterID, inviteeID)
	})
	callback := Callback{TradeID: "rollback-trade", OrderNo: orderNo, Amount: "2.00", ActualAmount: "2.00", ReceiveAddress: "T-rollback", Token: "usdt", TransactionID: "rollback-tx", Status: 2, Fields: map[string]string{"order_id": orderNo}}
	if err := (SQLRepository{DB: db, Invites: failingInviteReader{err: readercontract.ErrUnavailable}}).Process(ctx, callback); !errors.Is(err, readercontract.ErrUnavailable) {
		t.Fatalf("invite dependency err=%v", err)
	}
	if err := (SQLRepository{DB: db, Invites: readerprovider.NewInvite(db)}).Process(ctx, callback); err == nil {
		t.Fatal("expected inviter wallet overflow")
	}
	var status string
	var inviteeWallets, inviteeLedgers, rewards, callbacks int
	if err := db.QueryRowContext(ctx, `SELECT status FROM reader_recharge_orders WHERE id=$1`, orderID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallets WHERE reader_id=$1`, inviteeID).Scan(&inviteeWallets); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_wallet_ledgers WHERE reader_id=$1`, inviteeID).Scan(&inviteeLedgers); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_invite_reward_records WHERE invitee_reader_id=$1`, inviteeID).Scan(&rewards); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reader_payment_callback_logs WHERE recharge_order_id=$1`, orderID).Scan(&callbacks); err != nil {
		t.Fatal(err)
	}
	if status != "pending" || inviteeWallets != 0 || inviteeLedgers != 0 || rewards != 0 || callbacks != 0 {
		t.Fatalf("status=%s wallets=%d ledgers=%d rewards=%d callbacks=%d", status, inviteeWallets, inviteeLedgers, rewards, callbacks)
	}
}
