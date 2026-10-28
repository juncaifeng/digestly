package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/juncaifeng/digestly/core/model"
	"github.com/juncaifeng/digestly/core/opml"
	"github.com/juncaifeng/digestly/core/store"
)

// registerOPMLRoutes 订阅的 OPML 导入/导出。
func registerOPMLRoutes(r chi.Router, st *store.Store) {
	// 导出: 全部订阅源按专栏分组生成 OPML(供备份或迁移到其他阅读器)
	r.Get("/opml/export", func(w http.ResponseWriter, _ *http.Request) {
		feeds, err := st.ListFeeds()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		cols, err := st.ListColumns()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		nameOf := map[int64]string{}
		for _, c := range cols {
			nameOf[c.ID] = c.Name
		}
		groups := map[string][]opml.Entry{}
		var ungrouped []opml.Entry
		for _, f := range feeds {
			e := opml.Entry{Title: f.Title, XMLURL: f.URL}
			if f.ColumnID != nil {
				g := nameOf[*f.ColumnID]
				groups[g] = append(groups[g], e)
			} else {
				ungrouped = append(ungrouped, e)
			}
		}
		raw, err := opml.Build("digestly subscriptions", groups, ungrouped)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		w.Header().Set("Content-Type", "text/x-opml; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="digestly-subscriptions.opml"`)
		_, _ = w.Write(raw)
	})

	// 导入: 提交 OPML 文本,分组映射为专栏(不存在则建),订阅去重后新增
	r.Post("/opml/import", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			OPML string `json:"opml"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		entries, err := opml.Parse([]byte(body.OPML))
		if err != nil {
			writeErr(w, 400, err)
			return
		}

		existing := map[string]bool{}
		if feeds, err := st.ListFeeds(); err == nil {
			for _, f := range feeds {
				existing[f.URL] = true
			}
		}
		colIDs := map[string]int64{}
		if cols, err := st.ListColumns(); err == nil {
			for _, c := range cols {
				colIDs[c.Name] = c.ID
			}
		}

		var added, skipped, colsCreated int
		for _, e := range entries {
			if existing[e.XMLURL] {
				skipped++
				continue
			}
			title := e.Title
			if title == "" {
				title = e.XMLURL
			}
			f := &model.Feed{Title: title, URL: e.XMLURL, Collector: "rss", Config: "{}", IntervalMin: 30}
			if err := st.AddFeed(f); err != nil {
				writeErr(w, 500, err)
				return
			}
			if g := e.Group; g != "" {
				cid, ok := colIDs[g]
				if !ok {
					cid, err = st.AddColumn(g)
					if err != nil {
						writeErr(w, 500, err)
						return
					}
					colIDs[g] = cid
					colsCreated++
				}
				if err := st.SetFeedColumn(f.ID, &cid); err != nil {
					writeErr(w, 500, err)
					return
				}
			}
			existing[e.XMLURL] = true
			added++
		}
		writeJSON(w, 200, map[string]any{
			"added":          added,
			"skipped":        skipped, // 已订阅(按 URL 去重)
			"columns_created": colsCreated,
			"total":          len(entries),
		})
	})
}
