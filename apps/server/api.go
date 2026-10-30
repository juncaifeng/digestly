package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/juncaifeng/digestly/core/collector"
	"github.com/juncaifeng/digestly/core/model"
	"github.com/juncaifeng/digestly/core/scheduler"
	"github.com/juncaifeng/digestly/core/store"
)

// writeJSON 统一 JSON 出口。nil slice/map 序列化为 null,前端契约是空数组/对象,
// 这里统一兜底为 [] / {},避免各 handler 逐个防(曾两次导致生产白屏)。
func writeJSON(w http.ResponseWriter, code int, v any) {
	switch rv := reflect.ValueOf(v); {
	case rv.Kind() == reflect.Slice && rv.IsNil():
		v = reflect.MakeSlice(rv.Type(), 0, 0)
	case rv.Kind() == reflect.Map && rv.IsNil():
		v = reflect.MakeMap(rv.Type())
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func registerFeedRoutes(r chi.Router, st *store.Store, sched *scheduler.Scheduler) {
	r.Get("/feeds", func(w http.ResponseWriter, _ *http.Request) {
		feeds, err := st.ListFeeds()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, feeds)
	})
	r.Post("/feeds", func(w http.ResponseWriter, req *http.Request) {
		var f model.Feed
		if err := json.NewDecoder(req.Body).Decode(&f); err != nil {
			writeErr(w, 400, err)
			return
		}
		if f.Collector == "" {
			f.Collector = "rss"
		}
		if f.Config == "" {
			f.Config = "{}"
		}
		if f.IntervalMin <= 0 {
			f.IntervalMin = 30
		}
		if err := st.AddFeed(&f); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 201, f)
	})
	r.Delete("/feeds/{id}", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		if err := st.DeleteFeed(id); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(204)
	})
	// 设置历史上限(0 = 不限);只影响后续采集的入库量,不动存量(存量用归档管理)
	r.Patch("/feeds/{id}", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		var body struct {
			MaxItems *int `json:"max_items"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.MaxItems == nil || *body.MaxItems < 0 {
			writeErr(w, 400, fmt.Errorf("缺少 max_items(非负整数,0 = 不限)"))
			return
		}
		if err := st.SetFeedMaxItems(id, *body.MaxItems); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, map[string]int{"max_items": *body.MaxItems})
	})
	r.Post("/feeds/{id}/refresh", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		if err := sched.RefreshFeed(req.Context(), id); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(202)
	})
	// 批量归档该源的文章: scope = "read"(仅已读) / "all"(全部)
	r.Post("/feeds/{id}/archive", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		var body struct {
			Scope string `json:"scope"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		if body.Scope != "read" && body.Scope != "all" {
			writeErr(w, 400, fmt.Errorf("scope 只能是 read 或 all"))
			return
		}
		n, err := st.ArchiveFeedItems(id, body.Scope == "read")
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, map[string]int{"archived": n})
	})
}

func registerItemRoutes(r chi.Router, st *store.Store) {
	r.Get("/items", func(w http.ResponseWriter, req *http.Request) {
		q := req.URL.Query()
		feedID, _ := strconv.ParseInt(q.Get("feed_id"), 10, 64)
		columnID, _ := strconv.ParseInt(q.Get("column_id"), 10, 64)
		limit, _ := strconv.Atoi(q.Get("limit"))
		offset, _ := strconv.Atoi(q.Get("offset"))
		items, err := st.ListItems(store.ItemFilter{
			FeedID:   feedID,
			ColumnID: columnID,
			Tag:      q.Get("tag"),
			Query:    q.Get("q"),
			Unread:   q.Get("unread") == "1",
			Archived: q.Get("archived") == "1",
			Limit:    limit,
			Offset:   offset,
		})
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, items)
	})
	r.Get("/items/{id}", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		item, err := st.GetItem(id)
		if err != nil {
			writeErr(w, 404, err)
			return
		}
		writeJSON(w, 200, item)
	})
	r.Post("/items/{id}/read", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		var body struct {
			Read bool `json:"read"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		if err := st.MarkRead(id, body.Read); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(204)
	})
	// 归档/取消归档单篇(归档 = 从列表与未读数隐藏,不删除)
	r.Post("/items/{id}/archive", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		var body struct {
			Archived bool `json:"archived"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		if err := st.SetArchived(id, body.Archived); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(204)
	})
	r.Post("/items/{id}/tags", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		var body struct {
			Tag string `json:"tag"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.Tag == "" {
			writeErr(w, 400, http.ErrMissingFile)
			return
		}
		if err := st.AddTag(id, body.Tag); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(204)
	})
}

func registerCollectorRoutes(r chi.Router) {
	r.Get("/collectors", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, collector.Names())
	})
}

func registerStatsRoutes(r chi.Router, st *store.Store) {
	r.Get("/stats", func(w http.ResponseWriter, _ *http.Request) {
		items, entries, bytes := st.CacheStats()
		writeJSON(w, 200, map[string]any{
			"index_items":       items,
			"content_cached":    entries,
			"content_bytes":     bytes,
			"content_cap_bytes": store.DefaultContentCacheBytes,
		})
	})
}
