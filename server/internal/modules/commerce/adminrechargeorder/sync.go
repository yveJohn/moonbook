package adminrechargeorder

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/payment"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

var errSyncUnavailable = errors.New("payment sync is unavailable")

func (r SQLRepository) Sync(ctx context.Context, id int64) (Order, error) {
	if id <= 0 {
		return Order{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "ID必须是正整数字符串")
	}
	o, err := r.Get(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Order{}, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "充值订单不存在")
		}
		return Order{}, err
	}
	if o.Status == "paid" {
		return o, nil
	}
	if o.Provider != "epusdt" {
		return Order{}, apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, "当前支付渠道不支持主动同步")
	}
	if o.Status != "pending" && o.Status != "gateway_unknown" && o.Status != "callback_exception" {
		return Order{}, apperror.New(apperror.CodeConflict, http.StatusConflict, "当前订单状态不允许主动同步")
	}
	if r.PaymentRuntime == nil {
		return Order{}, apperror.New(apperror.CodeUnavailable, http.StatusServiceUnavailable, "支付主动同步未配置")
	}
	runtimeConfig, configErr := r.PaymentRuntime(ctx)
	if configErr != nil || runtimeConfig.EPUSDT.Credentials == nil {
		return Order{}, apperror.New(apperror.CodeUnavailable, http.StatusServiceUnavailable, "支付主动同步未配置")
	}
	credential := runtimeConfig.EPUSDT.Credentials.Current()
	pid := credential.PID()
	secret := credential.Secret()
	syncURL := strings.TrimSpace(runtimeConfig.SyncURL)
	parsed, parseErr := url.Parse(syncURL)
	if pid == "" || secret == "" || parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return Order{}, apperror.New(apperror.CodeUnavailable, http.StatusServiceUnavailable, "支付主动同步未配置")
	}
	requestFields := map[string]string{"pid": pid, "order_id": o.OrderNo}
	requestFields["signature"] = payment.Sign(requestFields, secret)
	body, _ := json.Marshal(requestFields)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, parsed.String(), strings.NewReader(string(body)))
	if err != nil {
		return Order{}, errSyncUnavailable
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: runtimeConfig.EPUSDT.RequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return Order{}, errSyncUnavailable
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16385))
	if err != nil || len(raw) == 0 || len(raw) > 16384 || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Order{}, errSyncUnavailable
	}
	fields, err := responseFields(raw)
	hash := sha256.Sum256(raw)
	payloadHash := hex.EncodeToString(hash[:])
	if err != nil || fields["pid"] != pid || fields["order_id"] != o.OrderNo || !payment.Verify(fields, fields["signature"], secret) {
		_ = r.recordSyncFailure(ctx, id, payloadHash, "响应签名或订单不匹配")
		return Order{}, apperror.New(apperror.CodeUnavailable, http.StatusBadGateway, "支付同步响应校验失败")
	}
	status, err := strconv.Atoi(fields["status"])
	if err != nil {
		_ = r.recordSyncFailure(ctx, id, payloadHash, "响应状态无效")
		return Order{}, apperror.New(apperror.CodeUnavailable, http.StatusBadGateway, "支付同步响应无效")
	}
	if status != 2 {
		if err = r.recordSyncPending(ctx, id, status, payloadHash); err != nil {
			return Order{}, err
		}
		return r.Get(ctx, id)
	}
	for _, key := range []string{"trade_id", "amount", "actual_amount", "receive_address", "token", "block_transaction_id"} {
		if strings.TrimSpace(fields[key]) == "" {
			_ = r.recordSyncFailure(ctx, id, payloadHash, "成功响应字段缺失")
			return Order{}, apperror.New(apperror.CodeUnavailable, http.StatusBadGateway, "支付同步响应无效")
		}
	}
	callback := payment.Callback{PID: fields["pid"], TradeID: fields["trade_id"], OrderNo: fields["order_id"], Amount: fields["amount"], ActualAmount: fields["actual_amount"], ReceiveAddress: fields["receive_address"], Token: fields["token"], TransactionID: fields["block_transaction_id"], Status: status, Fields: fields}
	if err = (payment.SQLRepository{DB: r.DB, Invites: r.Invites, Accounts: r.Accounts}).Process(ctx, callback); err != nil {
		_ = r.recordSyncFailure(ctx, id, payloadHash, "支付入账校验失败")
		return Order{}, apperror.New(apperror.CodeUnavailable, http.StatusBadGateway, "支付同步入账失败")
	}
	return r.Get(ctx, id)
}

func responseFields(raw []byte) (map[string]string, error) {
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawFields); err != nil {
		return nil, err
	}
	fields := make(map[string]string, len(rawFields))
	for key, value := range rawFields {
		var text string
		if json.Unmarshal(value, &text) != nil {
			var number json.Number
			if json.Unmarshal(value, &number) != nil {
				return nil, errors.New("invalid sync field")
			}
			text = number.String()
		}
		fields[key] = text
	}
	return fields, nil
}

func (r SQLRepository) recordSyncFailure(ctx context.Context, id int64, hash, reason string) error {
	return r.recordSync(ctx, id, nil, hash, "sync_rejected", reason)
}

func (r SQLRepository) recordSyncPending(ctx context.Context, id int64, status int, hash string) error {
	return r.recordSync(ctx, id, &status, hash, "sync_pending", "网关尚未完成支付")
}

func (r SQLRepository) recordSync(ctx context.Context, id int64, gatewayStatus *int, hash, result, reason string) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var orderNo string
	if err = tx.QueryRowContext(ctx, `SELECT order_no FROM reader_recharge_orders WHERE id=$1 FOR UPDATE`, id).Scan(&orderNo); err != nil {
		return err
	}
	status := "gateway_unknown"
	if result == "sync_rejected" {
		status = "callback_exception"
	}
	if _, err = tx.ExecContext(ctx, `UPDATE reader_recharge_orders SET gateway_status=COALESCE($1,gateway_status),status=$2,updated_at=now() WHERE id=$3 AND status <> 'paid'`, gatewayStatus, status, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO reader_payment_callback_logs(provider,recharge_order_id,merchant_order_no,payload_hash,signature_valid,processing_result,failure_reason,response_status,response_body,request_time) VALUES('epusdt',$1,$2,$3,false,$4,$5,200,'sync',now())`, id, orderNo, hash, result, reason); err != nil {
		return err
	}
	return tx.Commit()
}
