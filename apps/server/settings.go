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
	"github.com/juncaifeng/digestly/core/store"
)

func registerSettingsRoutes(r chi.Router, st *store.Store, dataDir string) {
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
		if strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.BaseURL) == "" || strings.TrimSpace(c.Model) == "" {
			writeErr(w, 400, fmt.Errorf("name / base_url / model 必填"))
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
		if strings.TrimSpace(c.Name) == "" || strings.TrimSpace(c.BaseURL) == "" || strings.TrimSpace(c.Model) == "" {
			writeErr(w, 400, fmt.Errorf("name / base_url / model 必填"))
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
			BaseURL: body.BaseURL, APIKey: body.APIKey,
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
		}
		reply, err := llm.Chat(req.Context(), cfg, "", "ping, reply with: pong")
		if err != nil {
			writeErr(w, 502, err)
			return
		}
		writeJSON(w, 200, map[string]string{"reply": reply})
	})
}
