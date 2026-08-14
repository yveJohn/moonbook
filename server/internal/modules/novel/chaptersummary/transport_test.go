package chaptersummary

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/aiconfig"
)

func TestSummaryMessageJSONAndSSE(t *testing.T) {
	jsonRaw := `{"choices":[{"message":{"content":"{\"chapter_summary\":\"主角出发\"}"}}]}`
	message, err := summaryMessage(jsonRaw, false)
	if err != nil || message != `{"chapter_summary":"主角出发"}` {
		t.Fatalf("JSON message=%q err=%v", message, err)
	}
	sseRaw := "data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"chapter_\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"summary\\\":\\\"重逢\\\"}\"}}]}\n\n" +
		"data: [DONE]\n"
	message, err = summaryMessage(sseRaw, true)
	if err != nil || message != `{"chapter_summary":"重逢"}` {
		t.Fatalf("SSE message=%q err=%v", message, err)
	}
}

func TestAITransportUsesJSONSchema(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local listener unavailable: %v", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/chat/completions" || request.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		format, _ := body["response_format"].(map[string]any)
		if format["type"] != "json_schema" {
			t.Fatalf("response_format=%v", format)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"{\"chapter_summary\":\"主角发现密道\"}"}}]}`))
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()
	summary, _, err := (AITransport{}).Summarize(context.Background(), aiconfig.RuntimeConfig{BaseURL: server.URL, ModelName: "fixture", StreamMode: "NON_STREAM"}, "prompt", "content", .1, 1000, time.Second)
	if err != nil || summary != "主角发现密道" {
		t.Fatalf("summary=%q err=%v", summary, err)
	}
}

func TestAITransportDetectsContentFilter(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local listener unavailable: %v", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(`{"choices":[{"finish_reason":"content_filter","message":{"content":""}}]}`))
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()
	_, _, err = (AITransport{}).Summarize(context.Background(), aiconfig.RuntimeConfig{BaseURL: server.URL, ModelName: "fixture"}, "prompt", "content", .1, 1000, time.Second)
	if !errors.Is(err, ErrAIRefusal) {
		t.Fatalf("err=%v", err)
	}
}

func TestTruncateRunesDoesNotSplitUnicode(t *testing.T) {
	if got := truncateRunes("章节摘要", 2); got != "章节" {
		t.Fatalf("got=%q", got)
	}
}
