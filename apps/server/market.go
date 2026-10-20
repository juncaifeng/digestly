package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/juncaifeng/digestly/core/collector"
	"github.com/juncaifeng/digestly/core/market"
	"github.com/juncaifeng/digestly/core/model"
	"github.com/juncaifeng/digestly/core/store"
)

func registerMarketRoutes(r chi.Router, mc *market.Client, fm *market.FeedMarket, st *store.Store, scriptsDir string) {
	// 市场清单 + 本地安装状态(读缓存,瞬时返回)
	r.Get("/market", func(w http.ResponseWriter, req *http.Request) {
		m, canWait := mc.List(req.Context())
		// 首轮加载(本地无清单)且前端要求等待: 挂起到后台预热完成(上限 5s),
		// 市场页首次打开即可看到内容;平时不带参数则纯缓存瞬时返回。
		if canWait && req.URL.Query().Get("wait_ready") == "1" {
			m = mc.WaitReady(req.Context(), 5*time.Second)
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
		stale, lastFetch := mc.Staleness()
		last := ""
		if !lastFetch.IsZero() {
			last = lastFetch.UTC().Format(time.RFC3339)
		}
		writeJSON(w, 200, map[string]any{"collectors": out, "stale": stale, "last_fetch": last})
	})

	// 解析第三方市场源: 只取地址里明确声明的订阅源清单(OPML / feeds.json),
	// 没有清单就报"没有"——不猜测、不把仓库 commits/releases 当订阅源。
	r.Post("/market/sources", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || strings.TrimSpace(body.URL) == "" {
			writeErr(w, 400, fmt.Errorf("缺少 url"))
			return
		}
		feeds, err := market.ParseFeeds(req.Context(), nil, strings.TrimSpace(body.URL))
		if err != nil {
			writeErr(w, 422, err)
			return
		}
		writeJSON(w, 200, map[string]any{"source": body.URL, "feeds": feeds, "count": len(feeds)})
	})

	// --- 第三方市场源(订阅源清单): 挂载后进市场,浏览/订阅都在市场页 ---

	// 列出已挂载市场源(含各自目录,读缓存瞬时返回)
	r.Get("/market/sources", func(w http.ResponseWriter, req *http.Request) {
		srcs, err := st.ListSources()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		type sourceWithFeeds struct {
			model.MarketSource
			Feeds     []market.MarketFeed `json:"feeds"`
			ParseErr  string              `json:"parse_err,omitempty"`
			FetchedAt string              `json:"fetched_at"`
		}
		out := make([]sourceWithFeeds, 0, len(srcs))
		for _, src := range srcs {
			sf := fm.Get(req.Context(), src.URL, src.Title)
			row := sourceWithFeeds{MarketSource: src, Feeds: sf.Feeds, ParseErr: sf.Err}
			if !sf.FetchedAt.IsZero() {
				row.FetchedAt = sf.FetchedAt.UTC().Format(time.RFC3339)
			}
			out = append(out, row)
		}
		writeJSON(w, 200, map[string]any{"sources": out})
	})

	// 挂载市场源: 立即解析一次,成功才入库
	r.Post("/market/sources", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			URL   string `json:"url"`
			Title string `json:"title"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || strings.TrimSpace(body.URL) == "" {
			writeErr(w, 400, fmt.Errorf("缺少 url"))
			return
		}
		raw := strings.TrimSpace(body.URL)
		sf := fm.Sync(req.Context(), raw, strings.TrimSpace(body.Title))
		if sf.Err != "" {
			writeErr(w, 422, fmt.Errorf("%s", sf.Err))
			return
		}
		title := body.Title
		if title == "" {
			title = raw
		}
		id, err := st.AddSource(raw, title)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		_ = st.TouchSource(id, len(sf.Feeds))
		writeJSON(w, 201, map[string]any{"id": id, "source": raw, "count": len(sf.Feeds), "feeds": sf.Feeds})
	})

	// 手动刷新某市场源目录
	r.Post("/market/sources/{id}/refresh", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		srcs, err := st.ListSources()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		for _, src := range srcs {
			if src.ID == id {
				sf := fm.Sync(req.Context(), src.URL, src.Title)
				if sf.Err != "" {
					writeErr(w, 422, fmt.Errorf("%s", sf.Err))
					return
				}
				_ = st.TouchSource(id, len(sf.Feeds))
				writeJSON(w, 200, map[string]any{"count": len(sf.Feeds)})
				return
			}
		}
		writeErr(w, 404, fmt.Errorf("market source %d not found", id))
	})

	// 卸载市场源
	r.Delete("/market/sources/{id}", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		srcs, _ := st.ListSources()
		for _, src := range srcs {
			if src.ID == id {
				fm.Evict(src.URL)
				break
			}
		}
		if err := st.DeleteSource(id); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(204)
	})

	// 显式刷新: 回源远端拉取最新清单
	r.Post("/market/refresh", func(w http.ResponseWriter, req *http.Request) {
		m, err := mc.Refresh(req.Context())
		if err != nil {
			writeErr(w, 502, err)
			return
		}
		writeJSON(w, 200, map[string]any{"refreshed": true, "collectors": len(m.Collectors)})
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
