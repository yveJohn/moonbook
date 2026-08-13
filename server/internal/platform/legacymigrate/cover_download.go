package legacymigrate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxLegacyCoverBytes = 10 << 20

type CoverDownloader struct {
	client       *http.Client
	allowedHosts map[string]struct{}
}

func NewCoverDownloader(timeout time.Duration, allowedHosts []string) (*CoverDownloader, error) {
	if timeout <= 0 || timeout > 2*time.Minute {
		return nil, errors.New("cover download timeout must be between 1ns and 2m")
	}
	allowed := make(map[string]struct{}, len(allowedHosts))
	for _, raw := range allowedHosts {
		host := strings.ToLower(strings.TrimSpace(raw))
		if host == "" || strings.ContainsAny(host, "/@") {
			return nil, fmt.Errorf("invalid allowed cover host %q", raw)
		}
		allowed[host] = struct{}{}
	}
	downloader := &CoverDownloader{allowedHosts: allowed}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = downloader.dialContext
	downloader.client = &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("legacy cover redirect limit exceeded")
			}
			return downloader.validateURL(request.Context(), request.URL)
		},
	}
	return downloader, nil
}

func (downloader *CoverDownloader) Download(ctx context.Context, rawURL string) ([]byte, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, errors.New("legacy cover URL is invalid")
	}
	if err := downloader.validateURL(ctx, parsed); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, errors.New("legacy cover request is invalid")
	}
	request.Header.Set("Accept", "image/jpeg,image/png,image/webp,image/gif")
	response, err := downloader.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download legacy cover: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("legacy cover returned HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxLegacyCoverBytes {
		return nil, errors.New("legacy cover exceeds 10 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxLegacyCoverBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read legacy cover: %w", err)
	}
	if len(data) == 0 || len(data) > maxLegacyCoverBytes {
		return nil, errors.New("legacy cover must contain 1 byte to 10 MiB")
	}
	return data, nil
}

func (downloader *CoverDownloader) validateURL(ctx context.Context, parsed *url.URL) error {
	if parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return errors.New("legacy cover URL must be HTTP(S) without user credentials")
	}
	host := strings.ToLower(parsed.Hostname())
	if downloader.hostAllowed(host) {
		return nil
	}
	port := parsed.Port()
	if port != "" && port != "80" && port != "443" {
		return errors.New("legacy cover URL uses a non-standard port")
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return errors.New("legacy cover host cannot be resolved")
	}
	for _, address := range addresses {
		if !publicAddress(address) {
			return errors.New("legacy cover host resolves to a non-public address")
		}
	}
	return nil
}

func (downloader *CoverDownloader) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, errors.New("legacy cover address is invalid")
	}
	if downloader.hostAllowed(strings.ToLower(host)) {
		return (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext(ctx, network, address)
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return nil, errors.New("legacy cover host cannot be resolved")
	}
	var failures []error
	for _, address := range addresses {
		if !publicAddress(address) {
			return nil, errors.New("legacy cover host resolves to a non-public address")
		}
		connection, dialErr := (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext(ctx, network, net.JoinHostPort(address.String(), port))
		if dialErr == nil {
			return connection, nil
		}
		failures = append(failures, dialErr)
	}
	return nil, errors.Join(failures...)
}

func (downloader *CoverDownloader) hostAllowed(host string) bool {
	_, ok := downloader.allowedHosts[host]
	return ok
}

func publicAddress(address netip.Addr) bool {
	return address.IsGlobalUnicast() && !address.IsPrivate() && !address.IsLoopback() && !address.IsLinkLocalUnicast() && !address.IsMulticast() && !address.IsUnspecified()
}

func ParseAllowedCoverHosts(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func ParseCoverDownloadTimeout(raw string) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return 20 * time.Second, nil
	}
	seconds, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || seconds < 1 || seconds > 120 {
		return 0, errors.New("MOONBOOK_LEGACY_COVER_TIMEOUT_SECONDS must be between 1 and 120")
	}
	return time.Duration(seconds) * time.Second, nil
}
