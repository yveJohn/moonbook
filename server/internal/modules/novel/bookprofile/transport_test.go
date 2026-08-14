package bookprofile

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/aiconfig"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func fixtureClient(contentType, body string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
}

func TestAITransportParsesJSONAndSSE(t *testing.T) {
	tests := []struct {
		name, streamMode, contentType, body string
	}{
		{"json", "NON_STREAM", "application/json", `{"choices":[{"message":{"content":"{\"book_name\":\"新书名\",\"book_desc\":\"新简介\",\"category_name\":\"玄幻\",\"sub_category_codes\":[\"hot\"]}"}}]}`},
		{"sse", "STREAM", "text/event-stream", "data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"book_name\\\":\\\"新书\\\",\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"\\\"book_desc\\\":\\\"简介\\\",\\\"category_code\\\":\\\"xuanhuan\\\"}\"}}]}\n\ndata: [DONE]\n\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, _, err := (AITransport{Client: fixtureClient(test.contentType, test.body)}).Generate(context.Background(), aiconfig.RuntimeConfig{BaseURL: "http://fixture", ModelName: "fixture", StreamMode: test.streamMode}, "prompt", `{}`, .1, 100, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if result.BookName == "" || result.BookDesc == "" {
				t.Fatalf("incomplete result: %+v", result)
			}
		})
	}
}

func TestAITransportDetectsRefusal(t *testing.T) {
	body := fmt.Sprintf(`{"choices":[{"finish_reason":"%s","message":{"content":""}}]}`, "content_filter")
	_, _, err := (AITransport{Client: fixtureClient("application/json", body)}).Generate(context.Background(), aiconfig.RuntimeConfig{BaseURL: "http://fixture", ModelName: "fixture"}, "prompt", `{}`, .1, 100, time.Second)
	if err != ErrAIRefusal {
		t.Fatalf("err=%v, want ErrAIRefusal", err)
	}
}
