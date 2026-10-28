// Package llm 提供 OpenAI 兼容(Chat Completions)的大模型调用客户端。
// 纯 net/http 实现,无外部依赖;base_url 形如 https://api.openai.com/v1。
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/juncaifeng/digestly/core/model"
)

var client = &http.Client{Timeout: 120 * time.Second}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat 调用一次对话补全,返回助手消息文本。cfg 需含明文 APIKey(store.GetLLMConfig / DefaultLLMConfig)。
func Chat(ctx context.Context, cfg *model.LLMConfig, system, user string) (string, error) {
	if cfg.BaseURL == "" || cfg.Model == "" {
		return "", fmt.Errorf("LLM 配置不完整: base_url 与 model 必填")
	}
	msgs := []message{}
	if system != "" {
		msgs = append(msgs, message{Role: "system", Content: system})
	}
	msgs = append(msgs, message{Role: "user", Content: user})

	body, err := json.Marshal(chatRequest{
		Model: cfg.Model, Messages: msgs,
		Temperature: cfg.Temperature, MaxTokens: cfg.MaxTokens,
	})
	if err != nil {
		return "", err
	}
	url := strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM 请求失败: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return "", err
	}
	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("LLM 响应解析失败(%d): %s", res.StatusCode, truncate(string(raw), 200))
	}
	if out.Error != nil {
		return "", fmt.Errorf("LLM 返回错误: %s", out.Error.Message)
	}
	if res.StatusCode != 200 {
		return "", fmt.Errorf("LLM HTTP %d: %s", res.StatusCode, truncate(string(raw), 200))
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("LLM 未返回任何内容")
	}
	content := strings.TrimSpace(out.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("LLM 返回空内容(思考型模型可能把 max_tokens 配额耗在思考上;可提高该配置的 max_tokens 或换非思考模型)")
	}
	return content, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
