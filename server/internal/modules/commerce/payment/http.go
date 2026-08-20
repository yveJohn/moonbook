package payment

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const callbackBodyLimit = 16 * 1024

type CallbackProcessor interface {
	ProcessAttempt(context.Context, int64, Callback) (AttemptResult, error)
}

type RequestMetadata struct {
	RequestID, TraceID, ClientIP string
}

type RequestMetadataProvider func(context.Context) RequestMetadata
type PanicObserver func(context.Context, int64)

type Handler struct {
	processor    CallbackProcessor
	audit        CallbackAuditRepository
	metadata     RequestMetadataProvider
	observePanic PanicObserver
	observer     AttemptObserver
}

func NewHandler(processor CallbackProcessor, audit CallbackAuditRepository, metadata RequestMetadataProvider, observePanic PanicObserver, observers ...AttemptObserver) *Handler {
	handler := &Handler{processor: processor, audit: audit, metadata: metadata, observePanic: observePanic}
	if len(observers) > 0 {
		handler.observer = observers[0]
	}
	return handler
}

func RegisterRoutes(group *gin.RouterGroup, handler *Handler) {
	group.POST("/reader/payment/epusdt/notify", handler.Handle)
}

func (handler *Handler) Handle(c *gin.Context) {
	body, truncated, readErr := readCallbackBody(c.Request.Body)
	meta := RequestMetadata{}
	if handler != nil && handler.metadata != nil {
		meta = handler.metadata(c.Request.Context())
	}
	start, err := NewAttemptStart(body, truncated, meta.RequestID, meta.TraceID, meta.ClientIP, time.Now().UTC())
	if err != nil || handler == nil || handler.audit == nil || handler.processor == nil {
		handler.writeResponse(c, http.StatusServiceUnavailable, "fail")
		return
	}
	attemptID, err := handler.audit.BeginAttempt(c.Request.Context(), start)
	if err != nil {
		handler.writeResponse(c, http.StatusServiceUnavailable, "fail")
		return
	}
	handler.observeAttempt(ResultReceived)
	defer handler.recoverAttempt(c, attemptID)

	if readErr != nil {
		handler.finishFailure(c, attemptID, FailureRequestRead, false, nil, nil)
		return
	}
	if truncated {
		handler.finishFailure(c, attemptID, FailurePayloadTooLarge, false, nil, nil)
		return
	}
	callback, err := parseCallback(body)
	if err != nil {
		handler.finishFailure(c, attemptID, FailureInvalidPayload, false, nil, nil)
		return
	}
	snapshot := SnapshotFromCallback(callback)
	result, err := handler.processor.ProcessAttempt(c.Request.Context(), attemptID, callback)
	if err != nil {
		code, signatureValid, orderID := processingErrorDetails(err)
		if code == "" {
			code = FailureDependency
		}
		handler.finishFailure(c, attemptID, code, signatureValid, orderID, &snapshot)
		return
	}
	handler.observeAttempt(result)
	handler.writeResponse(c, http.StatusOK, "success")
}

func readCallbackBody(reader io.Reader) ([]byte, bool, error) {
	if reader == nil {
		return nil, false, errors.New("callback body is unavailable")
	}
	body, err := io.ReadAll(io.LimitReader(reader, callbackBodyLimit+1))
	return body, len(body) == callbackBodyLimit+1, err
}

func parseCallback(body []byte) (Callback, error) {
	if len(body) == 0 {
		return Callback{}, errors.New("empty callback")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return Callback{}, errors.New("invalid callback JSON")
	}
	fields := make(map[string]string, len(raw))
	for key, value := range raw {
		var text string
		if len(value) == 0 || string(value) == "null" || json.Unmarshal(value, &text) != nil {
			var number json.Number
			if json.Unmarshal(value, &number) != nil {
				return Callback{}, errors.New("callback values must be scalar strings or numbers")
			}
			text = number.String()
		}
		fields[key] = text
	}
	for _, name := range []string{"pid", "trade_id", "order_id", "amount", "actual_amount", "receive_address", "token", "block_transaction_id", "status", "signature"} {
		if strings.TrimSpace(fields[name]) == "" {
			return Callback{}, errors.New("callback field is missing")
		}
	}
	status, err := strconv.Atoi(fields["status"])
	if err != nil {
		return Callback{}, errors.New("callback status is invalid")
	}
	return Callback{
		PID: fields["pid"], TradeID: fields["trade_id"], OrderNo: fields["order_id"], Amount: fields["amount"],
		ActualAmount: fields["actual_amount"], ReceiveAddress: fields["receive_address"], Token: fields["token"],
		TransactionID: fields["block_transaction_id"], Signature: fields["signature"], Status: status, Fields: fields,
	}, nil
}

func (handler *Handler) finishFailure(c *gin.Context, attemptID int64, code FailureCode, signatureValid bool, orderID *int64, snapshot *PayloadSnapshot) {
	definition, ok := failureDefinitions[code]
	if !ok {
		definition = failureDefinitions[FailureDependency]
		code = FailureDependency
	}
	completion := AttemptCompletion{Result: definition.result, FailureCode: code, ResponseStatus: definition.status, SignatureValid: signatureValid, OrderID: orderID, Snapshot: snapshot}
	if err := handler.audit.FinalizeAttempt(c.Request.Context(), attemptID, completion); err != nil {
		handler.writeResponse(c, http.StatusServiceUnavailable, "fail")
		return
	}
	handler.observeAttempt(definition.result)
	handler.writeResponse(c, definition.status, "fail")
}

func (handler *Handler) recoverAttempt(c *gin.Context, attemptID int64) {
	if recover() == nil {
		return
	}
	definition := failureDefinitions[FailurePanic]
	completion := AttemptCompletion{Result: definition.result, FailureCode: FailurePanic, ResponseStatus: definition.status}
	if handler.audit.FinalizeAttempt(c.Request.Context(), attemptID, completion) == nil {
		handler.observeAttempt(ResultFailed)
	}
	if handler.observePanic != nil {
		observePanicSafely(handler.observePanic, c.Request.Context(), attemptID)
	}
	handler.writeResponse(c, http.StatusServiceUnavailable, "fail")
}

func (handler *Handler) observeAttempt(result AttemptResult) {
	if handler != nil && handler.observer != nil {
		observeAttemptSafely(handler.observer, string(result))
	}
}

func (handler *Handler) writeResponse(c *gin.Context, status int, body string) {
	if handler != nil && handler.observer != nil {
		observeResponseSafely(handler.observer, status)
	}
	c.String(status, body)
}

func observeAttemptSafely(observer AttemptObserver, result string) {
	defer func() { _ = recover() }()
	observer.ObserveCallbackAttempt(result)
}

func observeResponseSafely(observer AttemptObserver, status int) {
	defer func() { _ = recover() }()
	observer.ObserveCallbackResponse(status)
}

func observePanicSafely(observer PanicObserver, ctx context.Context, attemptID int64) {
	defer func() { _ = recover() }()
	observer(ctx, attemptID)
}
