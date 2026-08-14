package bookprofile

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/modules/novel/aiconfig"
)

var ErrAIRefusal = errors.New("AI refused book profile content")

type Generator interface {
	Generate(context.Context, aiconfig.RuntimeConfig, string, string, float64, int, time.Duration) (SuggestedSnapshot, string, error)
}
type AITransport struct{ Client *http.Client }

func (t AITransport) Generate(ctx context.Context, runtime aiconfig.RuntimeConfig, prompt, input string, temperature float64, maxTokens int, timeout time.Duration) (SuggestedSnapshot, string, error) {
	body := map[string]any{"model": runtime.ModelName, "temperature": temperature, "stream": runtime.StreamMode == "STREAM", "messages": []map[string]string{{"role": "system", "content": prompt}, {"role": "user", "content": input}}, "response_format": map[string]any{"type": "json_object"}}
	if maxTokens > 0 {
		body["max_tokens"] = maxTokens
	}
	encoded, _ := json.Marshal(body)
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, strings.TrimRight(runtime.BaseURL, "/")+"/chat/completions", bytes.NewReader(encoded))
	if err != nil {
		return SuggestedSnapshot{}, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if runtime.APIKey() != "" {
		req.Header.Set("Authorization", "Bearer "+runtime.APIKey())
	}
	client := t.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(req)
	if err != nil {
		return SuggestedSnapshot{}, "", err
	}
	defer response.Body.Close()
	rawBytes, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return SuggestedSnapshot{}, "", err
	}
	raw := string(rawBytes)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return SuggestedSnapshot{}, raw, fmt.Errorf("AI HTTP %d", response.StatusCode)
	}
	if isRefusal(raw) {
		return SuggestedSnapshot{}, raw, ErrAIRefusal
	}
	content, err := profileMessage(raw, runtime.StreamMode == "STREAM" || strings.Contains(response.Header.Get("Content-Type"), "text/event-stream"))
	if err != nil {
		return SuggestedSnapshot{}, raw, err
	}
	content = extractJSON(content)
	var wire struct {
		BookName          string   `json:"book_name"`
		BookDesc          string   `json:"book_desc"`
		CategoryCode      string   `json:"category_code"`
		CategoryName      string   `json:"category_name"`
		CategoryReason    string   `json:"category_reason"`
		SubCategoryCodes  []string `json:"sub_category_codes"`
		SubCategoryNames  []string `json:"sub_category_names"`
		SubCategoryReason string   `json:"sub_category_reason"`
		Confidence        float64  `json:"confidence"`
		Reason            string   `json:"reason"`
	}
	if err = json.Unmarshal([]byte(content), &wire); err != nil {
		return SuggestedSnapshot{}, raw, fmt.Errorf("解析 AI 作品资料响应失败: %w", err)
	}
	wire.BookName = strings.TrimSpace(wire.BookName)
	wire.BookDesc = strings.TrimSpace(wire.BookDesc)
	if wire.BookName == "" || wire.BookDesc == "" {
		return SuggestedSnapshot{}, raw, errors.New("AI 响应缺少 book_name 或 book_desc")
	}
	result := SuggestedSnapshot{BookName: wire.BookName, BookDesc: wire.BookDesc, CategoryCode: strings.TrimSpace(wire.CategoryCode), CategoryName: strings.TrimSpace(wire.CategoryName), CategoryReason: wire.CategoryReason, SubCategoryReason: wire.SubCategoryReason, Confidence: wire.Confidence, Reason: wire.Reason, SubCategories: []SnapshotCategory{}}
	for i, code := range wire.SubCategoryCodes {
		name := ""
		if i < len(wire.SubCategoryNames) {
			name = wire.SubCategoryNames[i]
		}
		result.SubCategories = append(result.SubCategories, SnapshotCategory{Code: strings.TrimSpace(code), Name: strings.TrimSpace(name), Sort: i})
	}
	return result, raw, nil
}
func isRefusal(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.Contains(lower, "content_filter") || strings.Contains(lower, "i can't assist") || strings.Contains(lower, "i cannot assist") || strings.Contains(raw, "无法协助")
}
func profileMessage(raw string, stream bool) (string, error) {
	if stream {
		var output strings.Builder
		scanner := bufio.NewScanner(strings.NewReader(strings.ReplaceAll(raw, "\r\n", "\n")))
		scanner.Buffer(make([]byte, 1024), 8<<20)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || payload == "[DONE]" {
				continue
			}
			var event struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}
			if json.Unmarshal([]byte(payload), &event) == nil {
				for _, choice := range event.Choices {
					output.WriteString(choice.Delta.Content)
					output.WriteString(choice.Message.Content)
				}
			}
		}
		if output.Len() == 0 {
			return "", errors.New("AI 流式响应没有内容")
		}
		return output.String(), scanner.Err()
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(raw), &response); err != nil || len(response.Choices) == 0 {
		return "", errors.New("AI 响应没有消息内容")
	}
	return response.Choices[0].Message.Content, nil
}
func extractJSON(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "```json")
	value = strings.TrimPrefix(value, "```")
	value = strings.TrimSuffix(value, "```")
	start, end := strings.Index(value, "{"), strings.LastIndex(value, "}")
	if start >= 0 && end >= start {
		return value[start : end+1]
	}
	return value
}
