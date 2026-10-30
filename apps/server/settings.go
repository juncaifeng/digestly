package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/juncaifeng/digestly/core/scheduler"
	"github.com/juncaifeng/digestly/core/store"
)

// registerSettingsRoutes 应用设置: 采集入库限制(全局默认)。
func registerSettingsRoutes(r chi.Router, st *store.Store, dataDir string) {
	_ = dataDir // 预留: 用户自定义目录(如采集脚本)由上层处理

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
			MaxItems   *int `json:"max_items"`
			MaxAgeDays *int `json:"max_age_days"`
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
}
