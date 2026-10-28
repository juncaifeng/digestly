package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/juncaifeng/digestly/core/model"
)

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Embed 调用 OpenAI 兼容 /embeddings 接口,返回与输入等长的向量(按 index 归位)。
// cfg.Kind 应为 "embedding";api_key 可空(本地模型)。
func Embed(ctx context.Context, cfg *model.LLMConfig, texts []string) ([][]float32, error) {
	if cfg.BaseURL == "" || cfg.Model == "" {
		return nil, fmt.Errorf("embedding 配置不完整: base_url 与 model 必填")
	}
	if len(texts) == 0 {
		return nil, fmt.Errorf("输入为空")
	}
	body, err := json.Marshal(embedRequest{Model: cfg.Model, Input: texts})
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(cfg.BaseURL, "/") + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding 请求失败: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	var out embedResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("embedding 响应解析失败(%d): %s", res.StatusCode, truncate(string(raw), 200))
	}
	if out.Error != nil {
		return nil, fmt.Errorf("embedding 接口错误: %s", out.Error.Message)
	}
	if len(out.Data) != len(texts) {
		return nil, fmt.Errorf("embedding 返回条数不符: got %d want %d", len(out.Data), len(texts))
	}
	vecs := make([][]float32, len(texts))
	for _, d := range out.Data {
		if d.Index < 0 || d.Index >= len(vecs) || len(d.Embedding) == 0 {
			return nil, fmt.Errorf("embedding 返回数据异常")
		}
		vecs[d.Index] = d.Embedding
	}
	for i, v := range vecs {
		if v == nil {
			return nil, fmt.Errorf("embedding 缺第 %d 条向量", i)
		}
	}
	return vecs, nil
}
