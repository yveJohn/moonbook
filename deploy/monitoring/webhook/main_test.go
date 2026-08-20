package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestForwardDisabled(t *testing.T) {
	t.Setenv("MONITORING_WEBHOOK_FORWARD_URL", "")
	if err := forward([]byte(`{"status":"firing"}`)); err != nil {
		t.Fatal(err)
	}
}

func TestForwardUsesBearerToken(t *testing.T) {
	previousClient := webhookClient
	t.Cleanup(func() { webhookClient = previousClient })
	webhookClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization=%q", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"status":"resolved"}` {
			t.Fatalf("body=%s", body)
		}
		return &http.Response{StatusCode: http.StatusNoContent, Status: "204 No Content", Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	t.Setenv("MONITORING_WEBHOOK_FORWARD_URL", "https://alerts.example.test")
	t.Setenv("MONITORING_WEBHOOK_FORWARD_TOKEN", "secret")
	if err := forward([]byte(`{"status":"resolved"}`)); err != nil {
		t.Fatal(err)
	}
}
