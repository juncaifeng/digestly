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

	// 外部 Agent 运行时预设: 内置(embed) + <data>/llm_agents/*.yml 用户覆盖
	r.Get("/llm/agent-presets", func(w http.ResponseWriter, _ *http.Request) {
		presets, err := llm.AgentPresets(filepath.Join(dataDir, "llm_agents"))
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
		if err := normalizeLLMConfig(&c); err != nil {
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
		if err := normalizeLLMConfig(&c); err != nil {
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
			Command     string  `json:"command"`
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
			Kind: body.Kind, Command: body.Command,
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
			if body.Command == "" {
				cfg.Kind, cfg.Command = saved.Kind, saved.Command
			}
		}
		if cfg.Kind == "agent" {
			reply, err := llm.RunAgent(req.Context(), cfg.Command, "ping, 只回复: pong")
			if err != nil {
				writeErr(w, 502, err)
				return
			}
			writeJSON(w, 200, map[string]string{"reply": reply})
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

// normalizeLLMConfig 校验并补默认 kind。agent 需要 command;chat 需要 base_url + model。
func normalizeLLMConfig(c *model.LLMConfig) error {
	c.Kind = strings.TrimSpace(c.Kind)
	if c.Kind == "" {
		c.Kind = "chat"
	}
	if c.Kind != "chat" && c.Kind != "agent" {
		return fmt.Errorf("kind 只能是 chat 或 agent")
	}
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("name 必填")
	}
	if c.Kind == "agent" {
		if strings.TrimSpace(c.Command) == "" {
			return fmt.Errorf("agent 配置的 command 必填")
		}
		return nil
	}
	if strings.TrimSpace(c.BaseURL) == "" || strings.TrimSpace(c.Model) == "" {
		return fmt.Errorf("base_url / model 必填")
	}
	return nil
}
