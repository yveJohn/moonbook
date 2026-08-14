package adminpayment

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

type SQLRepository struct{ DB *sql.DB }

func (r SQLRepository) List(ctx context.Context) ([]Channel, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT id,provider,enabled,currency,token,network FROM reader_payment_channels ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	configured := os.Getenv("MOONBOOK_EPUSDT_PID") != "" && os.Getenv("MOONBOOK_EPUSDT_SECRET") != ""
	items := make([]Channel, 0)
	for rows.Next() {
		var v Channel
		if err := rows.Scan(&v.ID, &v.Provider, &v.Enabled, &v.Currency, &v.Token, &v.Network); err != nil {
			return nil, err
		}
		v.Configured = v.Provider != "epusdt" || configured
		items = append(items, v)
	}
	return items, rows.Err()
}

func (r SQLRepository) SetEnabled(ctx context.Context, id int64, enabled bool) (Channel, error) {
	var v Channel
	err := r.DB.QueryRowContext(ctx, `UPDATE reader_payment_channels SET enabled=$1,updated_at=now() WHERE id=$2 RETURNING id,provider,enabled,currency,token,network`, enabled, id).Scan(&v.ID, &v.Provider, &v.Enabled, &v.Currency, &v.Token, &v.Network)
	if err != nil {
		return Channel{}, err
	}
	v.Configured = v.Provider != "epusdt" || (os.Getenv("MOONBOOK_EPUSDT_PID") != "" && os.Getenv("MOONBOOK_EPUSDT_SECRET") != "")
	return v, nil
}

func (r SQLRepository) Check(ctx context.Context, id int64) (Connectivity, error) {
	var provider string
	var enabled bool
	if err := r.DB.QueryRowContext(ctx, `SELECT provider,enabled FROM reader_payment_channels WHERE id=$1`, id).Scan(&provider, &enabled); err != nil {
		if err == sql.ErrNoRows {
			return Connectivity{}, apperror.New(apperror.CodeNotFound, http.StatusNotFound, "支付渠道不存在")
		}
		return Connectivity{}, err
	}
	result := Connectivity{ChannelID: id, Provider: provider, CheckedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if !enabled {
		result.Status, result.Message = "disabled", "渠道未启用"
		return result, nil
	}
	switch provider {
	case "epusdt":
		if os.Getenv("MOONBOOK_EPUSDT_PID") == "" || os.Getenv("MOONBOOK_EPUSDT_SECRET") == "" {
			result.Status, result.Message = "not_configured", "渠道凭据未配置"
			return result, nil
		}
		healthURL := strings.TrimSpace(os.Getenv("MOONBOOK_EPUSDT_HEALTH_URL"))
		parsed, err := url.Parse(healthURL)
		if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			result.Status, result.Message = "not_configured", "健康检查地址未配置"
			return result, nil
		}
		result.Status, result.Message = probeHealth(ctx, healthURL)
		return result, nil
	default:
		result.Status, result.Message = "unsupported", "该渠道暂不支持连通性检查"
		return result, nil
	}
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
	_, _ = io.CopyN(io.Discard, resp.Body, 1024)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "reachable", "渠道可连通"
	}
	return "unreachable", fmt.Sprintf("渠道返回 HTTP %d", resp.StatusCode)
}
