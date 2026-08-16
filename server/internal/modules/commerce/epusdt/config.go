package epusdt

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	EnvPID                   = "MOONBOOK_EPUSDT_PID"
	EnvSecret                = "MOONBOOK_EPUSDT_SECRET"
	EnvCredentialRef         = "MOONBOOK_EPUSDT_CREDENTIAL_REF"
	EnvVerifyCredentialsJSON = "MOONBOOK_EPUSDT_VERIFY_CREDENTIALS_JSON"
	EnvCreateURL             = "MOONBOOK_EPUSDT_CREATE_URL"
	EnvNotifyURL             = "MOONBOOK_EPUSDT_NOTIFY_URL"
	EnvRedirectURL           = "MOONBOOK_EPUSDT_REDIRECT_URL"
	EnvConnectTimeoutMS      = "MOONBOOK_EPUSDT_CONNECT_TIMEOUT_MS"
	EnvRequestTimeoutMS      = "MOONBOOK_EPUSDT_REQUEST_TIMEOUT_MS"
	EnvUnknownReleaseMinutes = "MOONBOOK_EPUSDT_UNKNOWN_RELEASE_MINUTES"
)

const (
	defaultConnectTimeout      = 3 * time.Second
	defaultRequestTimeout      = 10 * time.Second
	defaultUnknownRelease      = 15 * time.Minute
	minimumHTTPTimeout         = 100 * time.Millisecond
	maximumConnectTimeout      = 30 * time.Second
	maximumRequestTimeout      = 2 * time.Minute
	minimumUnknownRelease      = time.Minute
	maximumUnknownRelease      = 24 * time.Hour
	maximumConfiguredURLLength = 2048
)

type LookupEnv func(string) (string, bool)

type Config struct {
	Enabled              bool
	Credentials          *CredentialProvider
	CreateURL            *url.URL
	NotifyURL            *url.URL
	RedirectURL          *url.URL
	ConnectTimeout       time.Duration
	RequestTimeout       time.Duration
	UnknownReleaseWindow time.Duration
}

func LoadConfig(enabled bool, lookup LookupEnv) (Config, error) {
	if !enabled {
		return Config{}, nil
	}
	if lookup == nil {
		lookup = func(string) (string, bool) { return "", false }
	}
	pid, err := requiredEnvironment(lookup, EnvPID, true)
	if err != nil {
		return Config{}, err
	}
	secret, err := requiredEnvironment(lookup, EnvSecret, false)
	if err != nil {
		return Config{}, err
	}
	credentialRef, err := requiredEnvironment(lookup, EnvCredentialRef, true)
	if err != nil {
		return Config{}, err
	}
	history, _ := lookup(EnvVerifyCredentialsJSON)
	credentials, err := NewCredentialProvider(credentialRef, pid, secret, history)
	if err != nil {
		return Config{}, fmt.Errorf("invalid EPUSDT credential configuration: %w", err)
	}

	createURL, err := configuredURL(lookup, EnvCreateURL)
	if err != nil {
		return Config{}, err
	}
	notifyURL, err := configuredURL(lookup, EnvNotifyURL)
	if err != nil {
		return Config{}, err
	}
	redirectURL, err := configuredURL(lookup, EnvRedirectURL)
	if err != nil {
		return Config{}, err
	}
	connectTimeout, err := configuredDuration(lookup, EnvConnectTimeoutMS, defaultConnectTimeout, minimumHTTPTimeout, maximumConnectTimeout, time.Millisecond)
	if err != nil {
		return Config{}, err
	}
	requestTimeout, err := configuredDuration(lookup, EnvRequestTimeoutMS, defaultRequestTimeout, minimumHTTPTimeout, maximumRequestTimeout, time.Millisecond)
	if err != nil {
		return Config{}, err
	}
	if requestTimeout < connectTimeout {
		return Config{}, fmt.Errorf("%s must be greater than or equal to %s", EnvRequestTimeoutMS, EnvConnectTimeoutMS)
	}
	unknownRelease, err := configuredDuration(lookup, EnvUnknownReleaseMinutes, defaultUnknownRelease, minimumUnknownRelease, maximumUnknownRelease, time.Minute)
	if err != nil {
		return Config{}, err
	}
	return Config{
		Enabled:              true,
		Credentials:          credentials,
		CreateURL:            createURL,
		NotifyURL:            notifyURL,
		RedirectURL:          redirectURL,
		ConnectTimeout:       connectTimeout,
		RequestTimeout:       requestTimeout,
		UnknownReleaseWindow: unknownRelease,
	}, nil
}

func requiredEnvironment(lookup LookupEnv, key string, trim bool) (string, error) {
	value, _ := lookup(key)
	if trim {
		value = strings.TrimSpace(value)
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required when EPUSDT is enabled", key)
	}
	return value, nil
}

func configuredURL(lookup LookupEnv, key string) (*url.URL, error) {
	raw, err := requiredEnvironment(lookup, key, true)
	if err != nil {
		return nil, err
	}
	if len(raw) > maximumConfiguredURLLength {
		return nil, fmt.Errorf("%s is invalid", key)
	}
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.Opaque != "" {
		return nil, fmt.Errorf("%s must be an absolute HTTP/HTTPS URL without user information or fragment", key)
	}
	return parsed, nil
}

func configuredDuration(lookup LookupEnv, key string, defaultValue, minimum, maximum, unit time.Duration) (time.Duration, error) {
	raw, _ := lookup(key)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 || value > int64(maximum/unit) {
		return 0, fmt.Errorf("%s is outside the supported range", key)
	}
	duration := time.Duration(value) * unit
	if duration < minimum || duration > maximum {
		return 0, fmt.Errorf("%s is outside the supported range", key)
	}
	return duration, nil
}
