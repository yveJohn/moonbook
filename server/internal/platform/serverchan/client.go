package serverchan

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	apiBaseURL     = "https://sctapi.ftqq.com/"
	maxResponseLen = 64 << 10
)

var sendKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

type Sender interface {
	Send(context.Context, string, string, string) error
}

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{httpClient: &http.Client{Timeout: 3 * time.Second}}
}

func newClientWithHTTP(client *http.Client) *Client {
	return &Client{httpClient: client}
}

func (client *Client) Send(ctx context.Context, sendKey, title, description string) error {
	if client == nil || client.httpClient == nil || !sendKeyPattern.MatchString(sendKey) {
		return ErrConfiguration
	}
	form := url.Values{"title": {title}, "desp": {description}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+sendKey+".send", strings.NewReader(form.Encode()))
	if err != nil {
		return ErrRequest
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return ErrRequest
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseLen+1))
	if err != nil || len(body) > maxResponseLen || response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return ErrResponse
	}
	var envelope struct {
		Code  *int `json:"code"`
		Errno *int `json:"errno"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return ErrResponse
	}
	if envelope.Code != nil && *envelope.Code == 0 || envelope.Errno != nil && *envelope.Errno == 0 {
		return nil
	}
	return ErrResponse
}
