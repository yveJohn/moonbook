package epusdt

import (
	"net/url"
	"time"
)

const DefaultUnknownReleaseWindow = 15 * time.Minute
const maximumConfiguredURLLength = 2048

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
