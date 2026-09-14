package serverchan

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestClientSendsFixedServerChanFormRequest(t *testing.T) {
	var captured *http.Request
	client := newClientWithHTTP(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		captured = request
		return response(http.StatusOK, `{"code":0,"message":"success"}`), nil
	})})
	if err := client.Send(context.Background(), "SCT_test_secret_123456", "标题", "第一行\n第二行"); err != nil {
		t.Fatal(err)
	}
	if captured.URL.String() != "https://sctapi.ftqq.com/SCT_test_secret_123456.send" || captured.Method != http.MethodPost {
		t.Fatalf("request=%s %s", captured.Method, captured.URL)
	}
	body, err := io.ReadAll(captured.Body)
	if err != nil {
		t.Fatal(err)
	}
	values, err := url.ParseQuery(string(body))
	if err != nil || values.Get("title") != "标题" || values.Get("desp") != "第一行\n第二行" {
		t.Fatalf("form=%q err=%v", body, err)
	}
}

func TestClientRejectsBadResponsesAndDoesNotLeakSendKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		do   roundTripFunc
		want error
	}{
		{name: "invalid key", key: "bad/key", do: func(*http.Request) (*http.Response, error) { return response(200, `{"code":0}`), nil }, want: ErrConfiguration},
		{name: "transport", key: "SCT_secret_transport", do: func(request *http.Request) (*http.Response, error) { return nil, fmt.Errorf("failed %s", request.URL) }, want: ErrRequest},
		{name: "http status", key: "SCT_secret_status", do: func(*http.Request) (*http.Response, error) { return response(502, `bad gateway`), nil }, want: ErrResponse},
		{name: "business", key: "SCT_secret_business", do: func(*http.Request) (*http.Response, error) { return response(200, `{"code":1,"message":"bad"}`), nil }, want: ErrResponse},
		{name: "invalid json", key: "SCT_secret_json", do: func(*http.Request) (*http.Response, error) { return response(200, `not-json`), nil }, want: ErrResponse},
		{name: "missing code", key: "SCT_secret_code", do: func(*http.Request) (*http.Response, error) { return response(200, `{}`), nil }, want: ErrResponse},
		{name: "oversized", key: "SCT_secret_large", do: func(*http.Request) (*http.Response, error) {
			return response(200, strings.Repeat("x", maxResponseLen+1)), nil
		}, want: ErrResponse},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newClientWithHTTP(&http.Client{Transport: test.do})
			err := client.Send(context.Background(), test.key, "title", "description")
			if !errors.Is(err, test.want) {
				t.Fatalf("err=%v want=%v", err, test.want)
			}
			if strings.Contains(err.Error(), test.key) {
				t.Fatalf("error leaked SendKey: %v", err)
			}
		})
	}
}

func TestClientMapsTimeoutWithoutLeakingSendKey(t *testing.T) {
	const sendKey = "SCT_timeout_secret_123456"
	client := newClientWithHTTP(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := client.Send(ctx, sendKey, "title", "description")
	if !errors.Is(err, ErrRequest) || strings.Contains(err.Error(), sendKey) {
		t.Fatalf("timeout err=%v", err)
	}
}
