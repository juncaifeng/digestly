package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// ListModels 调用 OpenAI 兼容 GET /models 拉取可用模型列表。
// 云端与本地运行时(Ollama / LM Studio / llama.cpp)同接口;api_key 可空。
func ListModels(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("base_url 不能为空")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(baseURL, "/")+"/models", nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("拉取模型列表失败: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("拉取模型列表失败(%d): %s", res.StatusCode, truncate(string(raw), 200))
	}
	var out modelsResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("模型列表解析失败: %s", truncate(string(raw), 200))
	}
	names := make([]string, 0, len(out.Data))
	for _, d := range out.Data {
		if d.ID != "" {
			names = append(names, d.ID)
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("接口没有返回模型")
	}
	return names, nil
}
