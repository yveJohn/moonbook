package chapterclean

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

type AIResult struct {
	ContentType        string  `json:"contentType"`
	IsNovelBody        bool    `json:"isNovelBody"`
	CleanedChapterName string  `json:"cleanedChapterName"`
	CleanedText        string  `json:"cleanedText"`
	ChapterSummary     string  `json:"chapterSummary"`
	RemovedNonNovel    bool    `json:"removedNonNovel"`
	Confidence         float64 `json:"confidence"`
}

type AITransport struct{ Client *http.Client }

var ErrAIRefusal = errors.New("AI refused chapter content")

func (transport AITransport) Call(ctx context.Context, runtime aiconfig.RuntimeConfig, prompt, chapterName, content string, temperature float64, maxTokens int, timeout time.Duration) (AIResult, string, error) {
	stream := runtime.StreamMode == "STREAM"
	body := map[string]any{"model": runtime.ModelName, "temperature": temperature, "stream": stream,
		"messages":        []map[string]string{{"role": "system", "content": prompt}, {"role": "user", "content": "章节名：" + chapterName + "\n\n正文：\n" + content}},
		"response_format": map[string]any{"type": "json_object"}}
	if maxTokens > 0 {
		body["max_tokens"] = maxTokens
	}
	encoded, _ := json.Marshal(body)
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(callCtx, http.MethodPost, strings.TrimRight(runtime.BaseURL, "/")+"/chat/completions", bytes.NewReader(encoded))
	if err != nil {
		return AIResult{}, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if runtime.APIKey() != "" {
		req.Header.Set("Authorization", "Bearer "+runtime.APIKey())
	}
	client := transport.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return AIResult{}, "", err
	}
	defer resp.Body.Close()
	rawBytes, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return AIResult{}, "", err
	}
	raw := string(rawBytes)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return AIResult{}, raw, fmt.Errorf("AI HTTP %d", resp.StatusCode)
	}
	if strings.Contains(raw, `"finish_reason":"content_filter"`) || strings.Contains(raw, `"finish_reason": "content_filter"`) {
		return AIResult{}, raw, ErrAIRefusal
	}
	message, err := extractAIMessage(raw, stream || strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream"))
	if err != nil {
		return AIResult{}, raw, err
	}
	message = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(message), "```json"), "```"))
	var result AIResult
	if err := json.Unmarshal([]byte(message), &result); err != nil {
		return AIResult{}, raw, fmt.Errorf("parse AI result: %w", err)
	}
	result.ContentType = strings.TrimSpace(result.ContentType)
	result.CleanedChapterName = strings.TrimSpace(result.CleanedChapterName)
	result.CleanedText = strings.TrimSpace(result.CleanedText)
	allowed := map[string]bool{"novel": true, "chat": true, "forum_noise": true, "advertisement": true, "mixed": true, "unknown": true}
	if !allowed[result.ContentType] {
		return AIResult{}, raw, errors.New("AI content type is invalid")
	}
	retained := result.ContentType == "novel" || result.ContentType == "mixed"
	result.IsNovelBody = retained
	if retained && (result.CleanedChapterName == "" || result.CleanedText == "" || strings.TrimSpace(result.ChapterSummary) == "") {
		return AIResult{}, raw, errors.New("AI result is incomplete")
	}
	return result, raw, nil
}

func extractAIMessage(raw string, stream bool) (string, error) {
	if stream {
		var out strings.Builder
		scanner := bufio.NewScanner(strings.NewReader(strings.ReplaceAll(raw, "\r\n", "\n")))
		scanner.Buffer(make([]byte, 1024), 8<<20)
		for scanner.Scan() {
			line := scanner.Text()
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
				} `json:"choices"`
			}
			if json.Unmarshal([]byte(payload), &event) == nil {
				for _, choice := range event.Choices {
					out.WriteString(choice.Delta.Content)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			return "", err
		}
		if out.Len() == 0 {
			return "", errors.New("AI stream contained no content")
		}
		return out.String(), nil
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
