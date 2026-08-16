package epusdt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxResponseBytes = 1_000_000
	maxRedirects     = 3
)

var (
	errRedirectRejected  = errors.New("EPUSDT redirect rejected")
	requestAmountPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,17})\.[0-9]{2}$`)
	orderIDPattern       = regexp.MustCompile(`^RC[1-9][0-9]{0,29}$`)
	decimalPattern       = regexp.MustCompile(`^(0|[1-9][0-9]*)(?:\.([0-9]+))?$`)
)

type Client struct {
	config     Config
	httpClient *http.Client
	now        func() time.Time
}

func NewClient(config Config) (*Client, error) {
	if !config.Enabled || config.Credentials == nil {
		return nil, errors.New("EPUSDT client configuration is disabled")
	}
	for name, configuredURL := range map[string]*url.URL{
		"create": config.CreateURL, "notify": config.NotifyURL, "redirect": config.RedirectURL,
	} {
		if !validAbsoluteHTTPURL(configuredURL, maximumConfiguredURLLength) {
			return nil, fmt.Errorf("EPUSDT %s URL is invalid", name)
		}
	}
	if config.ConnectTimeout <= 0 || config.RequestTimeout <= 0 || config.RequestTimeout < config.ConnectTimeout {
		return nil, errors.New("EPUSDT client timeouts are invalid")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: config.ConnectTimeout, KeepAlive: 30 * time.Second}).DialContext
	client := &Client{config: config, now: time.Now}
	client.httpClient = &http.Client{
		Transport: transport,
		Timeout:   config.RequestTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) > maxRedirects {
				return errRedirectRejected
			}
			origin := via[0].URL
			if request.URL.Scheme != origin.Scheme || !strings.EqualFold(request.URL.Host, origin.Host) {
				return errRedirectRejected
			}
			return nil
		},
	}
	return client, nil
}

func (client *Client) Create(ctx context.Context, request CreateRequest) (CreateResponse, error) {
	if err := validateCreateRequest(request); err != nil {
		return CreateResponse{}, err
	}
	credential := client.config.Credentials.Current()
	fields := map[string]string{
		"pid":          credential.PID(),
		"order_id":     request.OrderID,
		"currency":     "usd",
		"token":        "usdt",
		"network":      "tron",
		"amount":       request.Amount,
		"notify_url":   client.config.NotifyURL.String(),
		"redirect_url": client.config.RedirectURL.String(),
		"name":         "钻石充值",
	}
	fields["signature"] = Sign(fields, credential.Secret())
	form := make(url.Values, len(fields))
	for key, value := range fields {
		form.Set(key, value)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, client.config.CreateURL.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return CreateResponse{}, errors.New("EPUSDT create request is invalid")
	}
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpRequest.Header.Set("Accept", "application/json")

	httpResponse, err := client.httpClient.Do(httpRequest)
	if err != nil {
		if httpResponse != nil && httpResponse.Body != nil {
			_ = httpResponse.Body.Close()
		}
		switch {
		case errors.Is(err, errRedirectRejected):
			return CreateResponse{}, gatewayFailure("REDIRECT_REJECTED", FailureUncertain, "EPUSDT create redirect was rejected")
		case errors.Is(err, context.DeadlineExceeded) || isTimeout(err):
			return CreateResponse{}, gatewayFailure("REQUEST_TIMEOUT", FailureUncertain, "EPUSDT create request timed out")
		default:
			return CreateResponse{}, gatewayFailure("NETWORK_ERROR", FailureUncertain, "EPUSDT create request failed")
		}
	}
	defer httpResponse.Body.Close()
	body, err := io.ReadAll(io.LimitReader(httpResponse.Body, maxResponseBytes+1))
	if err != nil {
		return CreateResponse{}, gatewayFailure("RESPONSE_READ_ERROR", FailureUncertain, "EPUSDT create response could not be read")
	}
	if len(body) > maxResponseBytes {
		return CreateResponse{}, gatewayFailure("RESPONSE_TOO_LARGE", FailureUncertain, "EPUSDT create response exceeded the size limit")
	}
	envelope, err := decodeEnvelope(body)
	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		if err == nil && envelope.explicitRejection() {
			return CreateResponse{}, gatewayFailure("GATEWAY_REJECTED", FailureDefinite, "EPUSDT rejected the create request")
		}
		return CreateResponse{}, gatewayFailure("HTTP_ERROR", FailureUncertain, "EPUSDT create response used an unexpected HTTP status")
	}
	if err != nil {
		return CreateResponse{}, gatewayFailure("INVALID_RESPONSE", FailureUncertain, "EPUSDT create response was invalid")
	}
	if envelope.StatusCode != http.StatusOK {
		if envelope.explicitRejection() {
			return CreateResponse{}, gatewayFailure("GATEWAY_REJECTED", FailureDefinite, "EPUSDT rejected the create request")
		}
		return CreateResponse{}, gatewayFailure("INVALID_RESPONSE", FailureUncertain, "EPUSDT create response was inconsistent")
	}
	response, err := client.parseSuccess(envelope.Data, request)
	if err != nil {
		return CreateResponse{}, gatewayFailure("RESPONSE_MISMATCH", FailureUncertain, "EPUSDT create response did not match the local order")
	}
	return response, nil
}

func validateCreateRequest(request CreateRequest) error {
	if !orderIDPattern.MatchString(request.OrderID) {
		return errors.New("EPUSDT order ID is invalid")
	}
	if !requestAmountPattern.MatchString(request.Amount) {
		return errors.New("EPUSDT amount must be a positive decimal with two places")
	}
	amount, ok := new(big.Rat).SetString(request.Amount)
	if !ok || amount.Sign() <= 0 {
		return errors.New("EPUSDT amount must be positive")
	}
	return nil
}

type gatewayEnvelope struct {
	StatusCode int
	Data       json.RawMessage
}

func (envelope gatewayEnvelope) explicitRejection() bool {
	data := strings.TrimSpace(string(envelope.Data))
	return envelope.StatusCode != 0 && envelope.StatusCode != http.StatusOK && (data == "" || data == "null")
}

func decodeEnvelope(body []byte) (gatewayEnvelope, error) {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	var raw struct {
		StatusCode json.RawMessage `json:"status_code"`
		Data       json.RawMessage `json:"data"`
	}
	if err := decoder.Decode(&raw); err != nil {
		return gatewayEnvelope{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return gatewayEnvelope{}, errors.New("EPUSDT response contains trailing data")
	}
	statusCode, err := decodeInteger(raw.StatusCode)
	if err != nil {
		return gatewayEnvelope{}, err
	}
	return gatewayEnvelope{StatusCode: statusCode, Data: raw.Data}, nil
}

func (client *Client) parseSuccess(raw json.RawMessage, request CreateRequest) (CreateResponse, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return CreateResponse{}, errors.New("missing response data")
	}
	var data struct {
		TradeID        json.RawMessage `json:"trade_id"`
		OrderID        json.RawMessage `json:"order_id"`
		Amount         json.RawMessage `json:"amount"`
		Currency       json.RawMessage `json:"currency"`
		ActualAmount   json.RawMessage `json:"actual_amount"`
		ReceiveAddress json.RawMessage `json:"receive_address"`
		Token          json.RawMessage `json:"token"`
		Status         json.RawMessage `json:"status"`
		ExpirationTime json.RawMessage `json:"expiration_time"`
		PaymentURL     json.RawMessage `json:"payment_url"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return CreateResponse{}, err
	}
	tradeID, err := decodeText(data.TradeID)
	if err != nil || strings.TrimSpace(tradeID) != tradeID || tradeID == "" || utf8.RuneCountInString(tradeID) > 64 {
		return CreateResponse{}, errors.New("invalid trade ID")
	}
	orderID, err := decodeText(data.OrderID)
	if err != nil || orderID != request.OrderID {
		return CreateResponse{}, errors.New("order ID mismatch")
	}
	amountText, amount, err := decodeDecimal(data.Amount, 18, 8)
	if err != nil {
		return CreateResponse{}, err
	}
	expectedAmount, _ := new(big.Rat).SetString(request.Amount)
	if amount.Cmp(expectedAmount) != 0 {
		return CreateResponse{}, errors.New("amount mismatch")
	}
	currency, err := decodeText(data.Currency)
	if err != nil || currency != "usd" {
		return CreateResponse{}, errors.New("currency mismatch")
	}
	actualAmountText, actualAmount, err := decodeDecimal(data.ActualAmount, 16, 8)
	if err != nil || actualAmount.Sign() <= 0 {
		return CreateResponse{}, errors.New("invalid actual amount")
	}
	receiveAddress, err := decodeText(data.ReceiveAddress)
	if err != nil || strings.TrimSpace(receiveAddress) != receiveAddress || receiveAddress == "" || utf8.RuneCountInString(receiveAddress) > 255 {
		return CreateResponse{}, errors.New("invalid receive address")
	}
	token, err := decodeText(data.Token)
	if err != nil || token != "usdt" {
		return CreateResponse{}, errors.New("token mismatch")
	}
	status, err := decodeInteger(data.Status)
	if err != nil || status != 1 {
		return CreateResponse{}, errors.New("status mismatch")
	}
	expirationSeconds, err := decodeInt64(data.ExpirationTime)
	if err != nil || expirationSeconds <= client.now().Unix() {
		return CreateResponse{}, errors.New("invalid expiration time")
	}
	paymentURL, err := decodeText(data.PaymentURL)
	if err != nil || utf8.RuneCountInString(paymentURL) > 1000 {
		return CreateResponse{}, errors.New("invalid payment URL")
	}
	parsedPaymentURL, err := url.Parse(paymentURL)
	if err != nil || !validAbsoluteHTTPURL(parsedPaymentURL, 1000) {
		return CreateResponse{}, errors.New("invalid payment URL")
	}
	return CreateResponse{
		TradeID: tradeID, OrderID: orderID, Amount: amountText, Currency: currency,
		ActualAmount: actualAmountText, ReceiveAddress: receiveAddress, Token: token,
		Status: status, ExpirationTime: time.Unix(expirationSeconds, 0).UTC(), PaymentURL: paymentURL,
	}, nil
}

func decodeText(raw json.RawMessage) (string, error) {
	var value string
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return "", errors.New("expected text")
	}
	return value, nil
}

func decodeDecimal(raw json.RawMessage, maxIntegerDigits, maxScale int) (string, *big.Rat, error) {
	if len(raw) == 0 {
		return "", nil, errors.New("expected decimal")
	}
	text := strings.TrimSpace(string(raw))
	if strings.HasPrefix(text, `"`) {
		if err := json.Unmarshal(raw, &text); err != nil {
			return "", nil, errors.New("expected decimal")
		}
	}
	match := decimalPattern.FindStringSubmatch(text)
	if match == nil || len(match[1]) > maxIntegerDigits || len(match[2]) > maxScale {
		return "", nil, errors.New("decimal scale is invalid")
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return "", nil, errors.New("decimal is invalid")
	}
	return text, value, nil
}

func decodeInteger(raw json.RawMessage) (int, error) {
	value, err := decodeInt64(raw)
	if err != nil || int64(int(value)) != value {
		return 0, errors.New("integer is invalid")
	}
	return int(value), nil
}

func decodeInt64(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 {
		return 0, errors.New("expected integer")
	}
	text := strings.TrimSpace(string(raw))
	if strings.HasPrefix(text, `"`) {
		if err := json.Unmarshal(raw, &text); err != nil {
			return 0, errors.New("expected integer")
		}
	}
	return strconv.ParseInt(text, 10, 64)
}

func validAbsoluteHTTPURL(parsed *url.URL, maxLength int) bool {
	return parsed != nil && len(parsed.String()) <= maxLength && parsed.IsAbs() &&
		(parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" &&
		parsed.User == nil && parsed.Fragment == "" && parsed.Opaque == ""
}

func isTimeout(err error) bool {
	var networkError net.Error
	return errors.As(err, &networkError) && networkError.Timeout()
}

func gatewayFailure(code string, class FailureClass, summary string) *GatewayError {
	runes := []rune(strings.TrimSpace(summary))
	if len(runes) > 500 {
		runes = runes[:500]
	}
	return &GatewayError{Code: code, Class: class, Summary: string(runes)}
}
