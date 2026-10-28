package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/juncaifeng/digestly/core/llm"
	"github.com/juncaifeng/digestly/core/model"
	"github.com/juncaifeng/digestly/core/scheduler"
	"github.com/juncaifeng/digestly/core/store"
)

func registerSettingsRoutes(r chi.Router, st *store.Store, dataDir string) {
	// 采集入库限制(全局默认): 条数上限 + 天数上限,0 = 不限。
	// 只影响新拉取(入库前截断),已入库不动;每源 max_items 优先于全局条数。
	r.Get("/settings/ingest", func(w http.ResponseWriter, _ *http.Request) {
		mi := scheduler.DefaultIngestMaxItems
		ma := scheduler.DefaultIngestMaxAgeDays
		if v, ok, err := st.GetSetting("ingest_max_items"); err == nil && ok {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				mi = n
			}
		}
		if v, ok, err := st.GetSetting("ingest_max_age_days"); err == nil && ok {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				ma = n
			}
		}
		writeJSON(w, 200, map[string]int{"max_items": mi, "max_age_days": ma})
	})
	r.Put("/settings/ingest", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			MaxItems    *int `json:"max_items"`
			MaxAgeDays  *int `json:"max_age_days"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil ||
			body.MaxItems == nil || body.MaxAgeDays == nil || *body.MaxItems < 0 || *body.MaxAgeDays < 0 {
			writeErr(w, 400, fmt.Errorf("max_items / max_age_days 需为非负整数(0 = 不限)"))
			return
		}
		if err := st.SetSetting("ingest_max_items", strconv.Itoa(*body.MaxItems)); err != nil {
			writeErr(w, 500, err)
			return
		}
		if err := st.SetSetting("ingest_max_age_days", strconv.Itoa(*body.MaxAgeDays)); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, map[string]int{"max_items": *body.MaxItems, "max_age_days": *body.MaxAgeDays})
	})

	// 供应商预设: 内置(embed) + <data>/llm_providers/*.yml 用户覆盖
	r.Get("/llm/presets", func(w http.ResponseWriter, _ *http.Request) {
		presets, err := llm.Presets(filepath.Join(dataDir, "llm_providers"))
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, presets)
	})

	r.Get("/llm/configs", func(w http.ResponseWriter, _ *http.Request) {
		cfgs, err := st.ListLLMConfigs()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, cfgs)
	})

	r.Post("/llm/configs", func(w http.ResponseWriter, req *http.Request) {
		var c model.LLMConfig
		if err := json.NewDecoder(req.Body).Decode(&c); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := validateLLMConfig(&c); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := st.AddLLMConfig(&c); err != nil {
			writeErr(w, 500, err)
			return
		}
		c.APIKey = "" // 不回传密钥
		writeJSON(w, 201, c)
	})

	r.Patch("/llm/configs/{id}", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		var c model.LLMConfig
		if err := json.NewDecoder(req.Body).Decode(&c); err != nil {
			writeErr(w, 400, err)
			return
		}
		c.ID = id
		if err := validateLLMConfig(&c); err != nil {
			writeErr(w, 400, err)
			return
		}
		if err := st.UpdateLLMConfig(&c); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(204)
	})

	r.Delete("/llm/configs/{id}", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		if err := st.DeleteLLMConfig(id); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(204)
	})

	r.Post("/llm/configs/{id}/default", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		if err := st.SetDefaultLLMConfig(id); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(204)
	})

	// 连通性测试: id>0 用已存配置(含密钥),否则用请求体里的配置
	r.Post("/llm/test", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			ID          int64   `json:"id"`
			Kind        string  `json:"kind"`
			BaseURL     string  `json:"base_url"`
			APIKey      string  `json:"api_key"`
			Model       string  `json:"model"`
			Temperature float64 `json:"temperature"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		cfg := &model.LLMConfig{
			Kind: body.Kind, BaseURL: body.BaseURL, APIKey: body.APIKey,
			Model: body.Model, Temperature: body.Temperature, MaxTokens: 16,
		}
		if body.ID > 0 {
			saved, err := st.GetLLMConfig(body.ID)
			if err != nil {
				writeErr(w, 404, err)
				return
			}
			// 表单里没填新密钥(掩码或空)就沿用库里存的
			if body.APIKey == "" || strings.HasPrefix(body.APIKey, "****") {
				cfg.APIKey = saved.APIKey
			}
			if body.Kind == "" {
				cfg.Kind = saved.Kind
			}
			if cfg.BaseURL == "" {
				cfg.BaseURL = saved.BaseURL
			}
			if cfg.Model == "" {
				cfg.Model = saved.Model
			}
		}
		if cfg.Kind == "embedding" {
			vecs, err := llm.Embed(req.Context(), cfg, []string{"ping"})
			if err != nil {
				writeErr(w, 502, err)
				return
			}
			writeJSON(w, 200, map[string]string{"reply": fmt.Sprintf("连接成功,向量维度 %d", len(vecs[0]))})
			return
		}
		reply, err := llm.Chat(req.Context(), cfg, "", "ping, reply with: pong")
		if err != nil {
			writeErr(w, 502, err)
			return
		}
		writeJSON(w, 200, map[string]string{"reply": reply})
	})
}

// validateLLMConfig 校验并规范化(kind 空补 chat;两种 kind 都需要 base_url + model)。
func validateLLMConfig(c *model.LLMConfig) error {
	c.Kind = strings.TrimSpace(c.Kind)
	if c.Kind == "" {
		c.Kind = "chat"
	}
	if c.Kind != "chat" && c.Kind != "embedding" {
		return fmt.Errorf("kind 只能是 chat 或 embedding")
	}
	if strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.BaseURL) == "" || strings.TrimSpace(c.Model) == "" {
		return fmt.Errorf("name / base_url / model 必填")
	}
	return nil
}
