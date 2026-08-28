package adminpayment

import (
	"errors"
	"net/url"
	"strings"
)

const (
	createOrderPath = "/payments/gmpay/v1/order/create-transaction"
	notifyPath      = "/prod-api/reader/payment/epusdt/notify"
	redirectPath    = "/me/recharge"
	syncPath        = "/pay/check-status/{trade_id}"
)

type Endpoints struct {
	CreateURL   string
	NotifyURL   string
	RedirectURL string
	HealthURL   string
	SyncURL     string
}

func normalizeBaseURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 2048 {
		return "", errors.New("invalid base URL")
	}
	parsed, err := url.Parse(value)
	if err != nil || !parsed.IsAbs() || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", errors.New("invalid base URL")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", errors.New("invalid base URL")
	}
	parsed.Path, parsed.RawPath = "", ""
	return parsed.String(), nil
}

func deriveEndpoints(epusdtBaseURL, readerBaseURL string) (Endpoints, error) {
	epusdt, err := normalizeBaseURL(epusdtBaseURL)
	if err != nil {
		return Endpoints{}, err
	}
	reader, err := normalizeBaseURL(readerBaseURL)
	if err != nil {
		return Endpoints{}, err
	}
	return Endpoints{
		CreateURL: epusdt + createOrderPath, NotifyURL: reader + notifyPath,
		RedirectURL: reader + redirectPath, HealthURL: epusdt + "/", SyncURL: epusdt + syncPath,
	}, nil
}
