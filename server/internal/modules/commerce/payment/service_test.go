package payment

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/epusdt"
)

type verificationRepositoryStub struct {
	snapshot   VerificationSnapshot
	err        error
	processErr error
	processed  int
	attemptID  int64
}

func (stub *verificationRepositoryStub) VerificationSnapshot(context.Context, string) (VerificationSnapshot, error) {
	return stub.snapshot, stub.err
}

func (stub *verificationRepositoryStub) Process(context.Context, Callback) error {
	stub.processed++
	return nil
}

func (stub *verificationRepositoryStub) ProcessAttempt(_ context.Context, attemptID int64, _ Callback) (AttemptResult, error) {
	stub.processed++
	stub.attemptID = attemptID
	return ResultSuccess, stub.processErr
}

func TestServiceAttachesKnownOrderAndSignatureStateToFailures(t *testing.T) {
	credentials, err := epusdt.NewCredentialProvider("primary", "merchant", "secret", "[]")
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{"pid": "merchant", "order_id": "RC1", "trade_id": "trade", "amount": "2.00", "actual_amount": "2.00", "receive_address": "T-address", "token": "usdt", "block_transaction_id": "tx", "status": "2"}
	fields["signature"] = Sign(fields, "secret")
	callback := Callback{PID: "merchant", OrderNo: "RC1", Signature: fields["signature"], Fields: fields}

	repository := &verificationRepositoryStub{snapshot: VerificationSnapshot{OrderID: 9223372036854775000}, processErr: newProcessingError(FailureTransaction, ErrRejected)}
	_, err = NewService(repository, credentials).ProcessAttempt(context.Background(), 9, callback)
	code, signatureValid, orderID := processingErrorDetails(err)
	if code != FailureTransaction || !signatureValid || orderID == nil || *orderID != 9223372036854775000 {
		t.Fatalf("code=%s signature=%t orderID=%v err=%v", code, signatureValid, orderID, err)
	}

	repository = &verificationRepositoryStub{snapshot: VerificationSnapshot{OrderID: 9223372036854775001, CredentialRef: "missing"}}
	_, err = NewService(repository, credentials).ProcessAttempt(context.Background(), 10, callback)
	code, signatureValid, orderID = processingErrorDetails(err)
	if code != FailureUnknownCredential || signatureValid || orderID == nil || *orderID != 9223372036854775001 {
		t.Fatalf("code=%s signature=%t orderID=%v err=%v", code, signatureValid, orderID, err)
	}
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

func TestServiceClassifiesCallbackFailures(t *testing.T) {
	credentials, err := epusdt.NewCredentialProvider("primary", "merchant", "secret", "[]")
	if err != nil {
		t.Fatal(err)
	}
	validFields := map[string]string{"pid": "merchant", "order_id": "RC1", "trade_id": "trade", "amount": "2.00", "actual_amount": "2.00", "receive_address": "T-address", "token": "usdt", "block_transaction_id": "tx", "status": "2"}
	validFields["signature"] = Sign(validFields, "secret")
	valid := Callback{PID: "merchant", OrderNo: "RC1", Signature: validFields["signature"], Fields: validFields}

	tests := []struct {
		name       string
		repository *verificationRepositoryStub
		callback   Callback
		wantCode   FailureCode
	}{
		{name: "unknown order", repository: &verificationRepositoryStub{err: sql.ErrNoRows}, callback: valid, wantCode: FailureUnknownOrder},
		{name: "snapshot dependency", repository: &verificationRepositoryStub{err: errors.New("database unavailable")}, callback: valid, wantCode: FailureDependency},
		{name: "unknown credential", repository: &verificationRepositoryStub{snapshot: VerificationSnapshot{CredentialRef: "missing", MerchantPID: "merchant"}}, callback: valid, wantCode: FailureUnknownCredential},
		{name: "snapshot pid mismatch", repository: &verificationRepositoryStub{snapshot: VerificationSnapshot{CredentialRef: "primary", MerchantPID: "other"}}, callback: valid, wantCode: FailurePIDMismatch},
		{name: "callback pid mismatch", repository: &verificationRepositoryStub{snapshot: VerificationSnapshot{CredentialRef: "primary", MerchantPID: "merchant"}}, callback: func() Callback { value := valid; value.PID = "other"; return value }(), wantCode: FailurePIDMismatch},
		{name: "signature invalid", repository: &verificationRepositoryStub{snapshot: VerificationSnapshot{CredentialRef: "primary", MerchantPID: "merchant"}}, callback: func() Callback { value := valid; value.Signature = "invalid"; return value }(), wantCode: FailureSignatureInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewService(test.repository, credentials).ProcessAttempt(context.Background(), 91, test.callback)
			if FailureCodeOf(err) != test.wantCode || test.repository.processed != 0 {
				t.Fatalf("code=%s err=%v processed=%d", FailureCodeOf(err), err, test.repository.processed)
			}
		})
	}
}

func TestServicePassesAttemptIDToRepository(t *testing.T) {
	credentials, err := epusdt.NewCredentialProvider("primary", "merchant", "secret", "[]")
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{"pid": "merchant", "order_id": "RC1", "trade_id": "trade", "amount": "2.00", "actual_amount": "2.00", "receive_address": "T-address", "token": "usdt", "block_transaction_id": "tx", "status": "2"}
	fields["signature"] = Sign(fields, "secret")
	repository := &verificationRepositoryStub{}
	_, err = NewService(repository, credentials).ProcessAttempt(context.Background(), 9223372036854775000, Callback{PID: "merchant", OrderNo: "RC1", Signature: fields["signature"], Fields: fields})
	if err != nil || repository.attemptID != 9223372036854775000 {
		t.Fatalf("err=%v attemptID=%d", err, repository.attemptID)
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
