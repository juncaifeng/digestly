package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/juncaifeng/digestly/core/store"
)

func registerColumnRoutes(r chi.Router, st *store.Store) {
	r.Get("/columns", func(w http.ResponseWriter, _ *http.Request) {
		cols, err := st.ListColumns()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, cols)
	})
	r.Post("/columns", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
			writeErr(w, 400, fmt.Errorf("缺少 name"))
			return
		}
		id, err := st.AddColumn(strings.TrimSpace(body.Name))
		if err != nil {
			writeErr(w, 409, err) // 重名 UNIQUE 冲突
			return
		}
		writeJSON(w, 201, map[string]any{"id": id, "name": strings.TrimSpace(body.Name)})
	})
	r.Patch("/columns/{id}", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		if strings.TrimSpace(body.Name) != "" {
			if err := st.RenameColumn(id, strings.TrimSpace(body.Name)); err != nil {
				writeErr(w, 500, err)
				return
			}
		}
		w.WriteHeader(204)
	})
	r.Delete("/columns/{id}", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		if err := st.DeleteColumn(id); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(204)
	})

	// 移动订阅进/出专栏(column_id=0 或缺省 = 移回未分组)
	r.Post("/feeds/{id}/column", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		var body struct {
			ColumnID *int64 `json:"column_id"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		if body.ColumnID != nil && *body.ColumnID == 0 {
			body.ColumnID = nil
		}
		if err := st.SetFeedColumn(id, body.ColumnID); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(204)
	})

	// 手动收藏: 文章 <-> 专栏
	r.Post("/items/{id}/columns", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		var body struct {
			ColumnID int64 `json:"column_id"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.ColumnID <= 0 {
			writeErr(w, 400, fmt.Errorf("缺少 column_id"))
			return
		}
		if err := st.FavoriteItem(body.ColumnID, id); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(204)
	})
	r.Delete("/items/{id}/columns/{column_id}", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		colID, _ := strconv.ParseInt(chi.URLParam(req, "column_id"), 10, 64)
		if err := st.UnfavoriteItem(colID, id); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(204)
	})
}
