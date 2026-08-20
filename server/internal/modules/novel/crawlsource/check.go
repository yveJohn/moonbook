package crawlsource

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const (
	CheckCodeOK                  = "OK"
	CheckCodeAuthFailed          = "AUTH_FAILED"
	CheckCodeRedirectBlocked     = "REDIRECT_BLOCKED"
	CheckCodeTimeout             = "TIMEOUT"
	CheckCodeResponseTooLarge    = "RESPONSE_TOO_LARGE"
	CheckCodeDNSFailed           = "DNS_FAILED"
	CheckCodeConnectFailed       = "CONNECT_FAILED"
	CheckCodeHTTPError           = "HTTP_ERROR"
	CheckCodeTargetBlocked       = "TARGET_BLOCKED"
	CheckCodeSecretNotConfigured = "SECRET_NOT_CONFIGURED"
	defaultCheckMaxResponseBytes = 64 << 10
)

var (
	errCheckTargetBlocked   = errors.New("forum source target is blocked")
	errCheckDNSFailed       = errors.New("forum source DNS lookup failed")
	errCheckConnectFailed   = errors.New("forum source connection failed")
	errCheckRedirectBlocked = errors.New("forum source redirect is blocked")
)

type CheckResult struct {
	OK         bool
	Code       string
	HTTPStatus int
	ElapsedMs  int64
}

type CheckTarget struct {
	BaseURL, UserAgent, Cookie string
}

type SourceChecker interface {
	Check(context.Context, CheckTarget) CheckResult
}

type checkResolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type HTTPSourceChecker struct {
	client       *http.Client
	resolver     checkResolver
	allowedHosts map[string]struct{}
	maxBytes     int64
}

func NewHTTPSourceChecker(timeout time.Duration, maxBytes int64, allowedHosts []string) (*HTTPSourceChecker, error) {
	if timeout <= 0 || timeout > 30*time.Second {
		return nil, errors.New("forum source check timeout must be between 1ns and 30s")
	}
	if maxBytes <= 0 || maxBytes > 1<<20 {
		return nil, errors.New("forum source check response limit must be between 1 byte and 1 MiB")
	}
	checker := &HTTPSourceChecker{
		resolver:     net.DefaultResolver,
		allowedHosts: make(map[string]struct{}, len(allowedHosts)),
		maxBytes:     maxBytes,
	}
	for _, raw := range allowedHosts {
		host := strings.ToLower(strings.TrimSpace(raw))
		if host == "" || strings.ContainsAny(host, "/@:") {
			return nil, errors.New("invalid forum source check allowed host")
		}
		checker.allowedHosts[host] = struct{}{}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DisableKeepAlives = true
	transport.DialContext = checker.dialContext
	checker.client = &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) > 3 || len(via) == 0 || !strings.EqualFold(request.URL.Hostname(), via[0].URL.Hostname()) {
				return errCheckRedirectBlocked
			}
			if err := checker.validateURL(request.Context(), request.URL); err != nil {
				return errCheckRedirectBlocked
			}
			return nil
		},
	}
	return checker, nil
}

func NewDefaultHTTPSourceChecker() *HTTPSourceChecker {
	checker, err := NewHTTPSourceChecker(10*time.Second, defaultCheckMaxResponseBytes, nil)
	if err != nil {
		panic(err)
	}
	return checker
}

func (checker *HTTPSourceChecker) Check(ctx context.Context, target CheckTarget) (result CheckResult) {
	started := time.Now()
	result.Code = CheckCodeConnectFailed
	defer func() { result.ElapsedMs = time.Since(started).Milliseconds() }()

	parsed, err := url.Parse(strings.TrimSpace(target.BaseURL))
	if err != nil {
		result.Code = CheckCodeTargetBlocked
		return result
	}
	if err = checker.validateURL(ctx, parsed); err != nil {
		result.Code = classifyCheckError(err)
		return result
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		result.Code = CheckCodeTargetBlocked
		return result
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	if target.UserAgent != "" {
		request.Header.Set("User-Agent", target.UserAgent)
	}
	if target.Cookie != "" {
		request.Header.Set("Cookie", target.Cookie)
	}
	response, err := checker.client.Do(request)
	if err != nil {
		result.Code = classifyCheckError(err)
		return result
	}
	defer response.Body.Close()
	result.HTTPStatus = response.StatusCode
	if response.ContentLength > checker.maxBytes {
		result.Code = CheckCodeResponseTooLarge
		return result
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, checker.maxBytes+1))
	if readErr != nil {
		result.Code = classifyCheckError(readErr)
		return result
	}
	if int64(len(body)) > checker.maxBytes {
		result.Code = CheckCodeResponseTooLarge
		return result
	}
	switch {
	case response.StatusCode >= 200 && response.StatusCode < 300:
		result.OK, result.Code = true, CheckCodeOK
	case response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden:
		result.Code = CheckCodeAuthFailed
	default:
		result.Code = CheckCodeHTTPError
	}
	return result
}

func (checker *HTTPSourceChecker) validateURL(ctx context.Context, parsed *url.URL) error {
	if parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" {
		return errCheckTargetBlocked
	}
	host := strings.ToLower(parsed.Hostname())
	if checker.hostAllowed(host) {
		return nil
	}
	if port := parsed.Port(); port != "" && port != "80" && port != "443" {
		return errCheckTargetBlocked
	}
	addresses, err := checker.lookup(ctx, host)
	if err != nil {
		return err
	}
	for _, address := range addresses {
		if !publicCheckAddress(address) {
			return errCheckTargetBlocked
		}
	}
	return nil
}

func (checker *HTTPSourceChecker) lookup(ctx context.Context, host string) ([]netip.Addr, error) {
	if address, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{address}, nil
	}
	addresses, err := checker.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return nil, errCheckDNSFailed
	}
	return addresses, nil
}

func (checker *HTTPSourceChecker) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, errCheckTargetBlocked
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	if checker.hostAllowed(strings.ToLower(host)) {
		connection, dialErr := dialer.DialContext(ctx, network, address)
		if dialErr != nil {
			return nil, errors.Join(errCheckConnectFailed, dialErr)
		}
		return connection, nil
	}
	addresses, err := checker.lookup(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, resolved := range addresses {
		if !publicCheckAddress(resolved) {
			return nil, errCheckTargetBlocked
		}
		connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(resolved.String(), port))
		if dialErr == nil {
			return connection, nil
		}
	}
	return nil, errCheckConnectFailed
}

func (checker *HTTPSourceChecker) hostAllowed(host string) bool {
	_, ok := checker.allowedHosts[host]
	return ok
}

func publicCheckAddress(address netip.Addr) bool {
	return address.IsGlobalUnicast() && !address.IsPrivate() && !address.IsLoopback() && !address.IsLinkLocalUnicast() && !address.IsMulticast() && !address.IsUnspecified()
}

func classifyCheckError(err error) string {
	switch {
	case errors.Is(err, errCheckRedirectBlocked):
		return CheckCodeRedirectBlocked
	case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
		return CheckCodeTimeout
	case errors.Is(err, errCheckDNSFailed):
		return CheckCodeDNSFailed
	case errors.Is(err, errCheckTargetBlocked):
		return CheckCodeTargetBlocked
	default:
		return CheckCodeConnectFailed
	}
}
