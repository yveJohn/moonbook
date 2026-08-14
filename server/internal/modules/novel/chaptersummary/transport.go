package chaptersummary

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

var ErrAIRefusal = errors.New("AI refused chapter content")

type Summarizer interface {
	Summarize(context.Context, aiconfig.RuntimeConfig, string, string, float64, int, time.Duration) (string, string, error)
}

type AITransport struct{ Client *http.Client }

func (transport AITransport) Summarize(ctx context.Context, runtime aiconfig.RuntimeConfig, prompt, content string, temperature float64, maxTokens int, timeout time.Duration) (string, string, error) {
	body := map[string]any{
		"model": runtime.ModelName, "temperature": temperature, "stream": runtime.StreamMode == "STREAM",
		"messages": []map[string]string{{"role": "system", "content": prompt}, {"role": "user", "content": content}},
		"response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{
			"name": "chapter_summary_result", "strict": true,
			"schema": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"chapter_summary"}, "properties": map[string]any{"chapter_summary": map[string]string{"type": "string"}}},
		}},
	}
	if maxTokens > 0 {
		body["max_tokens"] = maxTokens
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return "", "", err
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, strings.TrimRight(runtime.BaseURL, "/")+"/chat/completions", bytes.NewReader(encoded))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if runtime.APIKey() != "" {
		req.Header.Set("Authorization", "Bearer "+runtime.APIKey())
	}
	client := transport.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	rawBytes, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return "", "", err
	}
	raw := string(rawBytes)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", raw, fmt.Errorf("AI HTTP %d", response.StatusCode)
	}
	if isRefusal(raw) {
		return "", raw, ErrAIRefusal
	}
	message, err := summaryMessage(raw, runtime.StreamMode == "STREAM" || strings.Contains(response.Header.Get("Content-Type"), "text/event-stream"))
	if err != nil {
		return "", raw, err
	}
	message = extractJSONObject(message)
	var result struct {
		ChapterSummary string `json:"chapter_summary"`
	}
	if err := json.Unmarshal([]byte(message), &result); err != nil {
		return "", raw, fmt.Errorf("parse AI summary: %w", err)
	}
	result.ChapterSummary = strings.TrimSpace(result.ChapterSummary)
	if result.ChapterSummary == "" {
		return "", raw, errors.New("chapter_summary is empty")
	}
	return result.ChapterSummary, raw, nil
}

func isRefusal(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.Contains(lower, `"finish_reason":"content_filter"`) || strings.Contains(lower, `"finish_reason": "content_filter"`) ||
		strings.Contains(lower, "i can't assist") || strings.Contains(lower, "i cannot assist") || strings.Contains(raw, "无法协助")
}

func summaryMessage(raw string, stream bool) (string, error) {
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
		if err := scanner.Err(); err != nil {
			return "", err
		}
		if output.Len() == 0 {
			return "", errors.New("AI stream contained no content")
		}
		return output.String(), nil
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		return "", err
	}
	if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
		return "", errors.New("AI response contained no content")
	}
	return response.Choices[0].Message.Content, nil
}

func extractJSONObject(content string) string {
	value := strings.TrimSpace(content)
	if start, end := strings.Index(value, "{"), strings.LastIndex(value, "}"); start >= 0 && end > start {
		return value[start : end+1]
	}
	return value
}
