package payment

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

type AttemptResult string

const (
	ResultReceived   AttemptResult = "received"
	ResultSuccess    AttemptResult = "success"
	ResultIdempotent AttemptResult = "idempotent"
	ResultRejected   AttemptResult = "rejected"
	ResultFailed     AttemptResult = "failed"
)

type FailureCode string

const (
	FailurePayloadTooLarge   FailureCode = "PAYLOAD_TOO_LARGE"
	FailureRequestRead       FailureCode = "REQUEST_READ_FAILED"
	FailureInvalidPayload    FailureCode = "INVALID_PAYLOAD"
	FailureUnknownOrder      FailureCode = "UNKNOWN_ORDER"
	FailureUnknownCredential FailureCode = "UNKNOWN_CREDENTIAL"
	FailurePIDMismatch       FailureCode = "PID_MISMATCH"
	FailureSignatureInvalid  FailureCode = "SIGNATURE_INVALID"
	FailureSnapshotMismatch  FailureCode = "SNAPSHOT_MISMATCH"
	FailureReplay            FailureCode = "REPLAY_DETECTED"
	FailureDependency        FailureCode = "DEPENDENCY_FAILED"
	FailureTransaction       FailureCode = "TRANSACTION_FAILED"
	FailurePanic             FailureCode = "PANIC"
)

var (
	ErrInvalidAttempt       = errors.New("invalid callback audit attempt")
	ErrAttemptStateConflict = errors.New("callback audit attempt is no longer received")
)

type AttemptStart struct {
	PayloadHash, SourceIPSHA256, RequestID, TraceID string
	PayloadBytes                                    int
	PayloadTruncated                                bool
	RequestTime                                     time.Time
}

type PayloadSnapshot struct {
	OrderNo        string `json:"order_id,omitempty"`
	TradeID        string `json:"trade_id,omitempty"`
	Amount         string `json:"amount,omitempty"`
	ActualAmount   string `json:"actual_amount,omitempty"`
	ReceiveAddress string `json:"receive_address,omitempty"`
	Token          string `json:"token,omitempty"`
	TransactionID  string `json:"block_transaction_id,omitempty"`
	Status         string `json:"status,omitempty"`
}

type AttemptCompletion struct {
	Result         AttemptResult
	FailureCode    FailureCode
	ResponseStatus int
	SignatureValid bool
	OrderID        *int64
	Snapshot       *PayloadSnapshot
}

type failureDefinition struct {
	result AttemptResult
	status int
	reason string
}

var failureDefinitions = map[FailureCode]failureDefinition{
	FailurePayloadTooLarge:   {ResultRejected, 400, "Callback payload exceeded the size limit"},
	FailureRequestRead:       {ResultFailed, 503, "Callback request could not be read"},
	FailureInvalidPayload:    {ResultRejected, 400, "Callback payload was invalid"},
	FailureUnknownOrder:      {ResultRejected, 400, "Recharge order was not found"},
	FailureUnknownCredential: {ResultRejected, 401, "Callback credential was not recognized"},
	FailurePIDMismatch:       {ResultRejected, 401, "Callback merchant identity did not match"},
	FailureSignatureInvalid:  {ResultRejected, 401, "Callback signature validation failed"},
	FailureSnapshotMismatch:  {ResultRejected, 400, "Callback did not match the order snapshot"},
	FailureReplay:            {ResultRejected, 400, "Callback reused a payment identifier"},
	FailureDependency:        {ResultFailed, 503, "Callback dependency was unavailable"},
	FailureTransaction:       {ResultFailed, 503, "Callback transaction could not be committed"},
	FailurePanic:             {ResultFailed, 503, "Callback processing stopped unexpectedly"},
}

func NewAttemptStart(payload []byte, truncated bool, requestID, traceID, sourceIP string, requestedAt time.Time) (AttemptStart, error) {
	if len(payload) > 16385 || truncated && len(payload) != 16385 || !validAuditID(requestID) || !validAuditID(traceID) || requestedAt.IsZero() {
		return AttemptStart{}, ErrInvalidAttempt
	}
	payloadHash := sha256.Sum256(payload)
	ipHash := sha256.Sum256([]byte(sourceIP))
	return AttemptStart{
		PayloadHash:      hex.EncodeToString(payloadHash[:]),
		SourceIPSHA256:   hex.EncodeToString(ipHash[:]),
		RequestID:        requestID,
		TraceID:          traceID,
		PayloadBytes:     len(payload),
		PayloadTruncated: truncated,
		RequestTime:      requestedAt,
	}, nil
}

func validAuditID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func SnapshotFromCallback(callback Callback) PayloadSnapshot {
	return PayloadSnapshot{
		OrderNo:        callback.OrderNo,
		TradeID:        callback.TradeID,
		Amount:         callback.Amount,
		ActualAmount:   callback.ActualAmount,
		ReceiveAddress: callback.ReceiveAddress,
		Token:          callback.Token,
		TransactionID:  callback.TransactionID,
		Status:         strconv.Itoa(callback.Status),
	}
}

func FailureReason(code FailureCode) (string, bool) {
	definition, ok := failureDefinitions[code]
	return definition.reason, ok
}

func validateAttemptCompletion(completion AttemptCompletion) error {
	switch completion.Result {
	case ResultSuccess, ResultIdempotent:
		if completion.FailureCode != "" || completion.ResponseStatus != 200 || !completion.SignatureValid || completion.OrderID == nil || *completion.OrderID <= 0 || completion.Snapshot == nil {
			return ErrInvalidAttempt
		}
		return nil
	case ResultRejected, ResultFailed:
		definition, ok := failureDefinitions[completion.FailureCode]
		if !ok || definition.result != completion.Result || definition.status != completion.ResponseStatus {
			return ErrInvalidAttempt
		}
		if completion.OrderID != nil && *completion.OrderID <= 0 {
			return ErrInvalidAttempt
		}
		return nil
	default:
		return ErrInvalidAttempt
	}
}

func (r SQLRepository) BeginAttempt(ctx context.Context, start AttemptStart) (int64, error) {
	if r.DB == nil || !validAuditID(start.RequestID) || !validAuditID(start.TraceID) || start.RequestTime.IsZero() || start.PayloadBytes < 0 || start.PayloadBytes > 16385 || start.PayloadTruncated && start.PayloadBytes != 16385 || !validSHA256(start.PayloadHash) || !validSHA256(start.SourceIPSHA256) {
		return 0, ErrInvalidAttempt
	}
	var id int64
	err := r.DB.QueryRowContext(ctx, `
INSERT INTO reader_payment_callback_logs
    (provider,payload_hash,signature_valid,processing_result,response_status,response_body,request_time,
     source_ip_sha256,source_type,request_id,trace_id,payload_bytes,payload_truncated)
VALUES ('epusdt',$1,false,'received',0,'',$2,$3,'runtime',$4,$5,$6,$7)
RETURNING id`, start.PayloadHash, start.RequestTime, start.SourceIPSHA256, start.RequestID, start.TraceID, start.PayloadBytes, start.PayloadTruncated).Scan(&id)
	return id, err
}

func (r SQLRepository) FinalizeAttempt(ctx context.Context, attemptID int64, completion AttemptCompletion) error {
	if r.DB == nil || completion.Result != ResultRejected && completion.Result != ResultFailed {
		return ErrInvalidAttempt
	}
	return finalizeAttempt(ctx, r.DB, attemptID, completion)
}

type auditExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func finalizeAttempt(ctx context.Context, executor auditExecutor, attemptID int64, completion AttemptCompletion) error {
	if executor == nil || attemptID <= 0 {
		return ErrInvalidAttempt
	}
	if err := validateAttemptCompletion(completion); err != nil {
		return err
	}
	var orderID any
	if completion.OrderID != nil {
		orderID = *completion.OrderID
	}
	var snapshotJSON any
	merchantOrderNo := ""
	gatewayTradeID := ""
	if completion.Snapshot != nil {
		encoded, err := json.Marshal(completion.Snapshot)
		if err != nil {
			return fmt.Errorf("encode callback audit snapshot: %w", err)
		}
		snapshotJSON = string(encoded)
		merchantOrderNo = completion.Snapshot.OrderNo
		gatewayTradeID = completion.Snapshot.TradeID
	}
	failureReason, _ := FailureReason(completion.FailureCode)
	responseBody := "fail"
	if completion.Result == ResultSuccess || completion.Result == ResultIdempotent {
		responseBody = "success"
	}
	result, err := executor.ExecContext(ctx, `
UPDATE reader_payment_callback_logs
SET recharge_order_id=$1,merchant_order_no=$2,gateway_trade_id=$3,payload_snapshot=$4::jsonb,
    signature_valid=$5,processing_result=$6,failure_code=NULLIF($7,''),failure_reason=$8,
    response_status=$9,response_body=$10,completed_at=now()
WHERE id=$11 AND processing_result='received'`,
		orderID, merchantOrderNo, gatewayTradeID, snapshotJSON, completion.SignatureValid, completion.Result,
		completion.FailureCode, failureReason, completion.ResponseStatus, responseBody, attemptID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrAttemptStateConflict
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
