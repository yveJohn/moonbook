package adminpayment

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/platform/apperror"
)

type Service struct{ Repo Repository }

func NewService(repo Repository) *Service { return &Service{Repo: repo} }
func (s *Service) List(ctx context.Context, includeArchived bool) ([]Channel, error) {
	return s.Repo.List(ctx, includeArchived)
}
func (s *Service) Get(ctx context.Context, id int64) (Channel, error) { return s.Repo.Get(ctx, id) }
func (s *Service) Create(ctx context.Context, input ChannelInput) (Channel, error) {
	if err := validateInput(input, true); err != nil {
		return Channel{}, err
	}
	return s.Repo.Create(ctx, input)
}
func (s *Service) Update(ctx context.Context, id int64, input ChannelInput) (Channel, error) {
	if id <= 0 {
		return Channel{}, invalid("ID必须是正整数字符串")
	}
	if err := validateInput(input, false); err != nil {
		return Channel{}, err
	}
	return s.Repo.Update(ctx, id, input)
}
func (s *Service) Archive(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("ID必须是正整数字符串")
	}
	return s.Repo.Archive(ctx, id)
}
func (s *Service) Check(ctx context.Context, id int64) (Connectivity, error) {
	channel, err := s.Repo.Get(ctx, id)
	if err != nil {
		return Connectivity{}, err
	}
	result := Connectivity{ChannelID: id, Provider: channel.Provider, CheckedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if channel.ArchivedAt != nil {
		result.Status, result.Message = "archived", "渠道已归档"
		return result, nil
	}
	if !channel.Enabled {
		result.Status, result.Message = "disabled", "渠道未启用"
		return result, nil
	}
	if !channel.Configured() {
		result.Status, result.Message = "not_configured", "渠道配置不完整"
		return result, nil
	}
	runtime, err := s.Repo.RuntimeForChannel(ctx, id)
	if err != nil {
		result.Status, result.Message = "not_configured", "渠道配置不可用"
		return result, nil
	}
	result.Status, result.Message = probeHealth(ctx, runtime.HealthURL)
	return result, nil
}

func validateInput(input ChannelInput, create bool) error {
	if strings.TrimSpace(input.DisplayName) == "" || len(input.DisplayName) > 100 {
		return invalid("渠道名称无效")
	}
	if input.Provider != "epusdt" || input.Currency != "usd" || input.Token != "usdt" || input.Network != "tron" {
		return invalid("当前仅支持EPUSDT的USD/USDT/TRON渠道")
	}
	if create && (input.MerchantPID == nil || input.Secret == nil) {
		return invalid("商户PID和Secret不能为空")
	}
	if input.MerchantPID != nil {
		value := strings.TrimSpace(*input.MerchantPID)
		if value == "" || len(value) > 255 {
			return invalid("商户PID无效")
		}
	}
	if input.Secret != nil && (strings.TrimSpace(*input.Secret) == "" || len(*input.Secret) > 4096) {
		return invalid("Secret无效")
	}
	if input.MerchantPID != nil && !create && input.Secret == nil {
		return invalid("修改商户PID时必须同时提交Secret")
	}
	for _, item := range []struct{ name, value string }{
		{name: "创建订单地址", value: input.CreateURL}, {name: "回调地址", value: input.NotifyURL},
		{name: "跳转地址", value: input.RedirectURL}, {name: "健康检查地址", value: input.HealthURL},
		{name: "订单同步地址", value: input.SyncURL},
	} {
		if err := validateURL(item.value); err != nil {
			return invalid(item.name + "无效")
		}
	}
	if input.ConnectTimeoutMS < 100 || input.ConnectTimeoutMS > 30000 || input.RequestTimeoutMS < input.ConnectTimeoutMS || input.RequestTimeoutMS > 120000 {
		return invalid("支付请求超时配置无效")
	}
	if input.UnknownReleaseMinutes < 1 || input.UnknownReleaseMinutes > 1440 {
		return invalid("未知订单释放时间无效")
	}
	return nil
}

func validateURL(value string) error {
	if strings.TrimSpace(value) != value || value == "" || len(value) > 2048 {
		return errors.New("invalid URL")
	}
	parsed, err := url.Parse(value)
	if err != nil || !parsed.IsAbs() || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.Opaque != "" {
		return errors.New("invalid URL")
	}
	return nil
}

func invalid(message string) error {
	return apperror.New(apperror.CodeInvalidArgument, http.StatusBadRequest, message)
}
