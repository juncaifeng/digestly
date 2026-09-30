package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/juncaifeng/digestly/core/collector"
	"github.com/juncaifeng/digestly/core/market"
	"github.com/juncaifeng/digestly/core/model"
	"github.com/juncaifeng/digestly/core/store"
)

func registerMarketRoutes(r chi.Router, mc *market.Client, st *store.Store, scriptsDir string) {
	// 市场清单 + 本地安装状态
	r.Get("/market", func(w http.ResponseWriter, req *http.Request) {
		m, err := mc.List(req.Context())
		if err != nil {
			writeErr(w, 502, err)
			return
		}
		installed := map[string]bool{}
		for _, n := range collector.Names() {
			if len(n) > 3 && n[:3] == "js:" {
				installed[n[3:]] = true
			}
		}
		versions, _ := st.MarketVersions()
		type entryWithState struct {
			market.Entry
			Installed        bool `json:"installed"`
			InstalledVersion int  `json:"installed_version"`
			UpdateAvailable  bool `json:"update_available"`
		}
		out := make([]entryWithState, 0, len(m.Collectors))
		for _, e := range m.Collectors {
			iv := versions[e.Name]
			out = append(out, entryWithState{
				e, installed[e.Name], iv,
				installed[e.Name] && e.ScriptVersion > iv,
			})
		}
		writeJSON(w, 200, map[string]any{"collectors": out})
	})

	// 安装: 下载脚本 -> 热重载 -> 自动建 feed(已存在同 URL 则跳过)
	r.Post("/market/install", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.Name == "" {
			writeErr(w, 400, err)
			return
		}
		entry, err := mc.Install(req.Context(), body.Name, scriptsDir)
		if err != nil {
			writeErr(w, 502, err)
			return
		}
		if err := collector.LoadJSScripts(); err != nil {
			writeErr(w, 500, err)
			return
		}
		_ = st.SetMarketVersion(entry.Name, entry.ScriptVersion)
		resp := map[string]any{"installed": entry.Name, "feed_created": false}
		feed := model.Feed{
			Title:       entry.Title,
			URL:         entry.SourceURL,
			Collector:   "js:" + entry.Name,
			Config:      market.DefaultConfig(entry),
			IntervalMin: 60,
		}
		if err := st.AddFeed(&feed); err == nil {
			resp["feed_created"] = true
			resp["feed_id"] = feed.ID
		}
		writeJSON(w, 201, resp)
	})
}
