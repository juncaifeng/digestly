package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/juncaifeng/digestly/core/collector"
	"github.com/juncaifeng/digestly/core/model"
	"github.com/juncaifeng/digestly/core/pipeline"
	"github.com/juncaifeng/digestly/core/scheduler"
	"github.com/juncaifeng/digestly/core/store"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
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
	r.Post("/feeds/{id}/refresh", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		if err := sched.RefreshFeed(req.Context(), id); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(202)
	})
}

func registerItemRoutes(r chi.Router, st *store.Store) {
	r.Get("/items", func(w http.ResponseWriter, req *http.Request) {
		q := req.URL.Query()
		feedID, _ := strconv.ParseInt(q.Get("feed_id"), 10, 64)
		limit, _ := strconv.Atoi(q.Get("limit"))
		offset, _ := strconv.Atoi(q.Get("offset"))
		items, err := st.ListItems(store.ItemFilter{
			FeedID: feedID,
			Status: model.ItemStatus(q.Get("status")),
			Tag:    q.Get("tag"),
			Query:  q.Get("q"),
			Unread: q.Get("unread") == "1",
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, items)
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

func registerPipelineRoutes(r chi.Router, chain *pipeline.Chain) {
	r.Post("/pipeline/run", func(w http.ResponseWriter, _ *http.Request) {
		n, err := chain.RunOnce(500)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, map[string]int{"processed": n})
	})
}
