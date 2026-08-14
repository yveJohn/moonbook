package chapterclean

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/aiconfig"
)

func TestExtractAIMessageJSON(t *testing.T) {
	raw := `{"choices":[{"message":{"content":"{\"contentType\":\"novel\"}"}}]}`
	got, err := extractAIMessage(raw, false)
	if err != nil || got != `{"contentType":"novel"}` {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestExtractAIMessageSSE(t *testing.T) {
	raw := "data: {\"choices\":[{\"delta\":{\"content\":\"{\\\"content\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"Type\\\":\\\"novel\\\"}\"}}]}\n\n" +
		"data: [DONE]\n"
	got, err := extractAIMessage(raw, true)
	if err != nil || got != `{"contentType":"novel"}` {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestAITransportCallsOpenAICompatibleEndpoint(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local listener unavailable: %v", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"contentType\":\"novel\",\"isNovelBody\":true,\"cleanedChapterName\":\"第一章\",\"cleanedText\":\"正文\",\"chapterSummary\":\"简介\"}"}}]}`))
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()
	result, _, err := (AITransport{}).Call(context.Background(), aiconfig.RuntimeConfig{BaseURL: server.URL, ModelName: "fixture", StreamMode: "NON_STREAM"}, "clean", "第一章", "广告正文", .1, 100, time.Second)
	if err != nil || result.CleanedText != "正文" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
