package adminpayment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/commerce/epusdt"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/secretcrypto"
)

const channelColumns = `id,display_name,provider,enabled,currency,token,network,(merchant_pid_ciphertext<>''),(secret_ciphertext<>''),create_url,notify_url,redirect_url,health_url,sync_url,connect_timeout_ms,request_timeout_ms,unknown_release_minutes,archived_at,created_at,updated_at`

type SQLRepository struct {
	DB     *sql.DB
	Cipher *secretcrypto.Cipher
}

func (r SQLRepository) List(ctx context.Context, includeArchived bool) ([]Channel, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT `+channelColumns+` FROM reader_payment_channels WHERE ($1 OR archived_at IS NULL) ORDER BY archived_at NULLS FIRST,id`, includeArchived)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Channel, 0)
	for rows.Next() {
		var item Channel
		if err := scanChannel(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r SQLRepository) Get(ctx context.Context, id int64) (Channel, error) {
	var item Channel
	err := scanChannel(r.DB.QueryRowContext(ctx, `SELECT `+channelColumns+` FROM reader_payment_channels WHERE id=$1`, id), &item)
	if errors.Is(err, sql.ErrNoRows) {
		return Channel{}, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "支付渠道不存在")
	}
	return item, err
}

func (r SQLRepository) Create(ctx context.Context, input ChannelInput) (Channel, error) {
	if r.DB == nil || r.Cipher == nil {
		return Channel{}, unavailable()
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return Channel{}, err
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO reader_payment_channels(display_name,provider,enabled,currency,token,network,create_url,notify_url,redirect_url,health_url,sync_url,connect_timeout_ms,request_timeout_ms,unknown_release_minutes) VALUES($1,$2,false,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) RETURNING id`, input.DisplayName, input.Provider, input.Currency, input.Token, input.Network, input.CreateURL, input.NotifyURL, input.RedirectURL, input.HealthURL, input.SyncURL, input.ConnectTimeoutMS, input.RequestTimeoutMS, input.UnknownReleaseMinutes).Scan(&id)
	if err != nil {
		return Channel{}, channelWriteError(err)
	}
	pidCipher, err := r.encrypt(id, "merchant_pid", strings.TrimSpace(*input.MerchantPID))
	if err != nil {
		return Channel{}, unavailable()
	}
	secretCipher, err := r.encrypt(id, "secret", *input.Secret)
	if err != nil {
		return Channel{}, unavailable()
	}
	if _, err = tx.ExecContext(ctx, `UPDATE reader_payment_channels SET merchant_pid_ciphertext=$1,secret_ciphertext=$2,enabled=$3,updated_at=now() WHERE id=$4`, pidCipher, secretCipher, input.Enabled, id); err != nil {
		return Channel{}, channelWriteError(err)
	}
	item, err := getChannelTx(ctx, tx, id)
	if err != nil {
		return Channel{}, err
	}
	if err = tx.Commit(); err != nil {
		return Channel{}, err
	}
	return item, nil
}

func (r SQLRepository) Update(ctx context.Context, id int64, input ChannelInput) (Channel, error) {
	if r.DB == nil || r.Cipher == nil {
		return Channel{}, unavailable()
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return Channel{}, err
	}
	defer tx.Rollback()
	var pidCipher, secretCipher string
	var wasEnabled bool
	var archivedAt sql.NullTime
	if err = tx.QueryRowContext(ctx, `SELECT merchant_pid_ciphertext,secret_ciphertext,enabled,archived_at FROM reader_payment_channels WHERE id=$1 FOR UPDATE`, id).Scan(&pidCipher, &secretCipher, &wasEnabled, &archivedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Channel{}, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "支付渠道不存在")
		}
		return Channel{}, err
	}
	if archivedAt.Valid {
		return Channel{}, apperror.New(apperror.CodeConflict, http.StatusConflict, "已归档渠道不能修改")
	}
	if input.MerchantPID != nil || input.Secret != nil || (wasEnabled && !input.Enabled) {
		if err = ensureNoActiveOrders(ctx, tx); err != nil {
			return Channel{}, err
		}
	}
	if input.MerchantPID != nil {
		pidCipher, err = r.encrypt(id, "merchant_pid", strings.TrimSpace(*input.MerchantPID))
		if err != nil {
			return Channel{}, unavailable()
		}
	}
	if input.Secret != nil {
		secretCipher, err = r.encrypt(id, "secret", *input.Secret)
		if err != nil {
			return Channel{}, unavailable()
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE reader_payment_channels SET display_name=$1,provider=$2,enabled=$3,currency=$4,token=$5,network=$6,merchant_pid_ciphertext=$7,secret_ciphertext=$8,create_url=$9,notify_url=$10,redirect_url=$11,health_url=$12,sync_url=$13,connect_timeout_ms=$14,request_timeout_ms=$15,unknown_release_minutes=$16,updated_at=now() WHERE id=$17`, input.DisplayName, input.Provider, input.Enabled, input.Currency, input.Token, input.Network, pidCipher, secretCipher, input.CreateURL, input.NotifyURL, input.RedirectURL, input.HealthURL, input.SyncURL, input.ConnectTimeoutMS, input.RequestTimeoutMS, input.UnknownReleaseMinutes, id)
	if err != nil {
		return Channel{}, channelWriteError(err)
	}
	item, err := getChannelTx(ctx, tx, id)
	if err != nil {
		return Channel{}, err
	}
	if err = tx.Commit(); err != nil {
		return Channel{}, err
	}
	return item, nil
}

func (r SQLRepository) Archive(ctx context.Context, id int64) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var archivedAt sql.NullTime
	if err = tx.QueryRowContext(ctx, `SELECT archived_at FROM reader_payment_channels WHERE id=$1 FOR UPDATE`, id).Scan(&archivedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperror.New(apperror.CodeNotFound, http.StatusNotFound, "支付渠道不存在")
		}
		return err
	}
	if archivedAt.Valid {
		return nil
	}
	if err = ensureNoActiveOrders(ctx, tx); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE reader_payment_channels SET enabled=false,merchant_pid_ciphertext='',secret_ciphertext='',archived_at=now(),updated_at=now() WHERE id=$1`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (r SQLRepository) Runtime(ctx context.Context) (RuntimeConfig, error) {
	return r.runtime(ctx, `provider='epusdt' AND enabled=true AND archived_at IS NULL`, nil)
}

func (r SQLRepository) RuntimeForChannel(ctx context.Context, id int64) (RuntimeConfig, error) {
	return r.runtime(ctx, `id=$1 AND enabled=true AND archived_at IS NULL`, []any{id})
}

func (r SQLRepository) UnknownReleaseWindow(ctx context.Context) (time.Duration, error) {
	var minutes int
	err := r.DB.QueryRowContext(ctx, `SELECT unknown_release_minutes FROM reader_payment_channels WHERE provider='epusdt' AND enabled=true AND archived_at IS NULL`).Scan(&minutes)
	if errors.Is(err, sql.ErrNoRows) {
		return epusdt.DefaultUnknownReleaseWindow, nil
	}
	if err != nil {
		return 0, err
	}
	return time.Duration(minutes) * time.Minute, nil
}

func (r SQLRepository) runtime(ctx context.Context, where string, args []any) (RuntimeConfig, error) {
	if r.DB == nil || r.Cipher == nil {
		return RuntimeConfig{}, unavailable()
	}
	var id int64
	var pidCipher, secretCipher, createURL, notifyURL, redirectURL, healthURL, syncURL string
	var connectMS, requestMS, unknownMinutes int
	err := r.DB.QueryRowContext(ctx, `SELECT id,merchant_pid_ciphertext,secret_ciphertext,create_url,notify_url,redirect_url,health_url,sync_url,connect_timeout_ms,request_timeout_ms,unknown_release_minutes FROM reader_payment_channels WHERE `+where, args...).Scan(&id, &pidCipher, &secretCipher, &createURL, &notifyURL, &redirectURL, &healthURL, &syncURL, &connectMS, &requestMS, &unknownMinutes)
	if err != nil {
		return RuntimeConfig{}, unavailable()
	}
	pid, err := r.decrypt(id, "merchant_pid", pidCipher)
	if err != nil {
		return RuntimeConfig{}, unavailable()
	}
	secret, err := r.decrypt(id, "secret", secretCipher)
	if err != nil {
		return RuntimeConfig{}, unavailable()
	}
	credentials, err := epusdt.NewCredentialProvider("channel-"+strconv.FormatInt(id, 10), pid, secret, "[]")
	if err != nil {
		return RuntimeConfig{}, unavailable()
	}
	parsed := make([]*url.URL, 3)
	for index, raw := range []string{createURL, notifyURL, redirectURL} {
		if validateURL(raw) != nil {
			return RuntimeConfig{}, unavailable()
		}
		parsed[index], _ = url.Parse(raw)
	}
	return RuntimeConfig{ChannelID: id, EPUSDT: epusdt.Config{
		Enabled: true, Credentials: credentials, CreateURL: parsed[0], NotifyURL: parsed[1], RedirectURL: parsed[2],
		ConnectTimeout: time.Duration(connectMS) * time.Millisecond, RequestTimeout: time.Duration(requestMS) * time.Millisecond,
		UnknownReleaseWindow: time.Duration(unknownMinutes) * time.Minute,
	}, HealthURL: healthURL, SyncURL: syncURL}, nil
}

func (r SQLRepository) encrypt(id int64, field, plaintext string) (string, error) {
	return r.Cipher.Encrypt(plaintext, secretcrypto.Scope{Table: "reader_payment_channels", RecordID: strconv.FormatInt(id, 10), Field: field})
}
func (r SQLRepository) decrypt(id int64, field, ciphertext string) (string, error) {
	return r.Cipher.Decrypt(ciphertext, secretcrypto.Scope{Table: "reader_payment_channels", RecordID: strconv.FormatInt(id, 10), Field: field})
}

func ensureNoActiveOrders(ctx context.Context, tx *sql.Tx) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM reader_recharge_orders WHERE provider='epusdt' AND status IN ('creating','pending','gateway_unknown','callback_exception'))`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return apperror.New(apperror.CodeConflict, http.StatusConflict, "存在进行中的支付订单，不能修改凭据或归档渠道")
	}
	return nil
}

func getChannelTx(ctx context.Context, tx *sql.Tx, id int64) (Channel, error) {
	var item Channel
	err := scanChannel(tx.QueryRowContext(ctx, `SELECT `+channelColumns+` FROM reader_payment_channels WHERE id=$1`, id), &item)
	return item, err
}
func scanChannel(row interface{ Scan(...any) error }, item *Channel) error {
	return row.Scan(&item.ID, &item.DisplayName, &item.Provider, &item.Enabled, &item.Currency, &item.Token, &item.Network, &item.PIDConfigured, &item.SecretConfigured, &item.CreateURL, &item.NotifyURL, &item.RedirectURL, &item.HealthURL, &item.SyncURL, &item.ConnectTimeoutMS, &item.RequestTimeoutMS, &item.UnknownReleaseMinutes, &item.ArchivedAt, &item.CreatedAt, &item.UpdatedAt)
}
func channelWriteError(err error) error {
	if strings.Contains(err.Error(), "reader_payment_channels_active_provider_uidx") {
		return apperror.New(apperror.CodeConflict, http.StatusConflict, "EPUSDT渠道已存在")
	}
	return err
}
func unavailable() error {
	return apperror.New(apperror.CodeUnavailable, http.StatusServiceUnavailable, "支付配置不可用")
}

func probeHealth(ctx context.Context, healthURL string) (string, string) {
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, healthURL, nil)
	if err != nil {
		return "unreachable", "健康检查请求失败"
	}
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		return "unreachable", "健康检查请求失败"
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "reachable", "渠道可连通"
	}
	return "unreachable", fmt.Sprintf("渠道返回 HTTP %d", resp.StatusCode)
}
