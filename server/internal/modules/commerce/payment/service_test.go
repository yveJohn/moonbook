package payment

import (
	"context"
	"errors"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/epusdt"
)

type verificationRepositoryStub struct {
	snapshot  VerificationSnapshot
	err       error
	processed int
}

func (stub *verificationRepositoryStub) VerificationSnapshot(context.Context, string) (VerificationSnapshot, error) {
	return stub.snapshot, stub.err
}

func (stub *verificationRepositoryStub) Process(context.Context, Callback) error {
	stub.processed++
	return nil
}

func TestServiceVerifiesCallbackWithOrderCredential(t *testing.T) {
	credentials, err := epusdt.NewCredentialProvider("primary", "merchant-current", "current-secret", `[{"ref":"previous","pid":"merchant-previous","secret":"previous-secret"}]`)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		snapshot   VerificationSnapshot
		pid        string
		secret     string
		wantReject bool
	}{
		{name: "current", snapshot: VerificationSnapshot{CredentialRef: "primary", MerchantPID: "merchant-current"}, pid: "merchant-current", secret: "current-secret"},
		{name: "historical", snapshot: VerificationSnapshot{CredentialRef: "previous", MerchantPID: "merchant-previous"}, pid: "merchant-previous", secret: "previous-secret"},
		{name: "legacy empty snapshot", pid: "merchant-current", secret: "current-secret"},
		{name: "unknown reference", snapshot: VerificationSnapshot{CredentialRef: "missing", MerchantPID: "merchant-current"}, pid: "merchant-current", secret: "current-secret", wantReject: true},
		{name: "snapshot pid mismatch", snapshot: VerificationSnapshot{CredentialRef: "previous", MerchantPID: "merchant-current"}, pid: "merchant-current", secret: "previous-secret", wantReject: true},
		{name: "callback pid mismatch", snapshot: VerificationSnapshot{CredentialRef: "primary", MerchantPID: "merchant-current"}, pid: "other", secret: "current-secret", wantReject: true},
		{name: "wrong signature", snapshot: VerificationSnapshot{CredentialRef: "primary", MerchantPID: "merchant-current"}, pid: "merchant-current", secret: "wrong-secret", wantReject: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &verificationRepositoryStub{snapshot: test.snapshot}
			fields := map[string]string{"pid": test.pid, "order_id": "RC1", "trade_id": "trade", "amount": "2.00", "actual_amount": "2.00", "receive_address": "T-address", "token": "usdt", "block_transaction_id": "tx", "status": "2"}
			fields["signature"] = Sign(fields, test.secret)
			callback := Callback{PID: test.pid, OrderNo: "RC1", Signature: fields["signature"], Fields: fields}
			err := NewService(repository, credentials).Process(context.Background(), callback)
			if test.wantReject {
				if !errors.Is(err, ErrUnauthorized) || repository.processed != 0 {
					t.Fatalf("err=%v processed=%d", err, repository.processed)
				}
				return
			}
			if err != nil || repository.processed != 1 {
				t.Fatalf("err=%v processed=%d", err, repository.processed)
			}
		})
	}
}

func TestServiceDoesNotProcessWhenSnapshotLookupFails(t *testing.T) {
	credentials, err := epusdt.NewCredentialProvider("primary", "merchant", "secret", "[]")
	if err != nil {
		t.Fatal(err)
	}
	repository := &verificationRepositoryStub{err: errors.New("database unavailable")}
	err = NewService(repository, credentials).Process(context.Background(), Callback{OrderNo: "RC1"})
	if err == nil || repository.processed != 0 {
		t.Fatalf("err=%v processed=%d", err, repository.processed)
	}
}
