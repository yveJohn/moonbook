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
	"strings"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

const (
	syncResponseLimit = 16 * 1024
	syncTradeIDToken  = "{trade_id}"
)

var errSyncUnavailable = errors.New("payment sync is unavailable")

type gmPayStatusEnvelope struct {
	StatusCode int `json:"status_code"`
	Data       *struct {
		TradeID string `json:"trade_id"`
		Status  int    `json:"status"`
	} `json:"data"`
}

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
	if strings.TrimSpace(o.GatewayTradeID) == "" {
		return Order{}, apperror.New(apperror.CodeConflict, http.StatusConflict, "订单缺少网关交易号，无法同步")
	}
	if r.PaymentRuntime == nil {
		return Order{}, apperror.New(apperror.CodeUnavailable, http.StatusServiceUnavailable, "支付主动同步未配置")
	}
	runtimeConfig, configErr := r.PaymentRuntime(ctx)
	if configErr != nil || runtimeConfig.EPUSDT.Credentials == nil || runtimeConfig.EPUSDT.RequestTimeout <= 0 {
		return Order{}, apperror.New(apperror.CodeUnavailable, http.StatusServiceUnavailable, "支付主动同步未配置")
	}
	syncURL, err := buildSyncURL(runtimeConfig.SyncURL, o.GatewayTradeID)
	if err != nil {
		return Order{}, apperror.New(apperror.CodeUnavailable, http.StatusServiceUnavailable, "支付主动同步未配置")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, syncURL, nil)
	if err != nil {
		return Order{}, errSyncUnavailable
	}
	req.Header.Set("Accept", "application/json")
	resp, err := (&http.Client{Timeout: runtimeConfig.EPUSDT.RequestTimeout}).Do(req)
	if err != nil {
		_ = r.recordSyncFailure(ctx, id, hashSyncFailure("network_error"), "网关状态请求失败")
		return Order{}, errSyncUnavailable
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, syncResponseLimit+1))
	payloadHash := hashSyncPayload(raw)
	if err != nil || len(raw) == 0 || len(raw) > syncResponseLimit || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = r.recordSyncFailure(ctx, id, payloadHash, "网关状态响应无效")
		return Order{}, errSyncUnavailable
	}
	status, err := parseGMStatus(raw, o.GatewayTradeID)
	if err != nil {
		_ = r.recordSyncFailure(ctx, id, payloadHash, "网关状态响应校验失败")
		return Order{}, apperror.New(apperror.CodeUnavailable, http.StatusBadGateway, "支付同步响应无效")
	}
	result, reason, targetStatus, clearActive := syncOutcome(status)
	if err = r.recordSyncOutcome(ctx, id, status, payloadHash, result, reason, targetStatus, clearActive); err != nil {
		return Order{}, err
	}
	return r.Get(ctx, id)
}

func buildSyncURL(template, tradeID string) (string, error) {
	template = strings.TrimSpace(template)
	if strings.Count(template, syncTradeIDToken) != 1 || strings.TrimSpace(tradeID) == "" {
		return "", errors.New("invalid sync URL template")
	}
	raw := strings.Replace(template, syncTradeIDToken, url.PathEscape(tradeID), 1)
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", errors.New("invalid sync URL")
	}
	return raw, nil
}

func parseGMStatus(raw []byte, expectedTradeID string) (int, error) {
	var envelope gmPayStatusEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.StatusCode != http.StatusOK || envelope.Data == nil || envelope.Data.TradeID != expectedTradeID {
		return 0, errors.New("invalid GM Pay status response")
	}
	if envelope.Data.Status < 1 || envelope.Data.Status > 4 {
		return 0, errors.New("unknown GM Pay order status")
	}
	return envelope.Data.Status, nil
}

func syncOutcome(status int) (result, reason, targetStatus string, clearActive bool) {
	switch status {
	case 1:
		return "sync_pending", "网关等待支付", "pending", false
	case 2:
		return "paid_no_callback", "网关已支付但缺少有效签名回调，请在GM Pay重发回调", "callback_exception", false
	case 3:
		return "sync_expired", "网关订单已过期", "expired", true
	case 4:
		return "sync_select", "网关等待选择支付网络或币种", "gateway_unknown", false
	default:
		return "sync_rejected", "未知网关状态", "gateway_unknown", false
	}
}

func hashSyncPayload(raw []byte) string {
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

func hashSyncFailure(code string) string { return hashSyncPayload([]byte(code)) }

func (r SQLRepository) recordSyncFailure(ctx context.Context, id int64, hash, reason string) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var orderNo, status string
	if err = tx.QueryRowContext(ctx, `SELECT order_no,status FROM reader_recharge_orders WHERE id=$1 FOR UPDATE`, id).Scan(&orderNo, &status); err != nil {
		return err
	}
	if status == "paid" {
		return tx.Commit()
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO reader_payment_callback_logs(provider,recharge_order_id,merchant_order_no,payload_hash,signature_valid,processing_result,failure_reason,response_status,response_body,request_time) VALUES('epusdt',$1,$2,$3,false,'sync_rejected',$4,502,'sync',now())`, id, orderNo, hash, reason); err != nil {
		return err
	}
	return tx.Commit()
}

func (r SQLRepository) recordSyncOutcome(ctx context.Context, id int64, gatewayStatus int, hash, result, reason, targetStatus string, clearActive bool) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var orderNo, status string
	if err = tx.QueryRowContext(ctx, `SELECT order_no,status FROM reader_recharge_orders WHERE id=$1 FOR UPDATE`, id).Scan(&orderNo, &status); err != nil {
		return err
	}
	if status == "paid" {
		return tx.Commit()
	}
	if _, err = tx.ExecContext(ctx, `UPDATE reader_recharge_orders SET gateway_status=$1,status=$2,active_reader_id=CASE WHEN $3 THEN NULL ELSE active_reader_id END,updated_at=now() WHERE id=$4`, gatewayStatus, targetStatus, clearActive, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO reader_payment_callback_logs(provider,recharge_order_id,merchant_order_no,payload_hash,signature_valid,processing_result,failure_reason,response_status,response_body,request_time) VALUES('epusdt',$1,$2,$3,false,$4,$5,200,'sync',now())`, id, orderNo, hash, result, reason); err != nil {
		return err
	}
	return tx.Commit()
}
