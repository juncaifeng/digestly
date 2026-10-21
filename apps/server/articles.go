package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/juncaifeng/digestly/core/model"
	"github.com/juncaifeng/digestly/core/store"
)

func registerArticleRoutes(r chi.Router, st *store.Store) {
	// --- 模板 ---
	r.Get("/templates", func(w http.ResponseWriter, _ *http.Request) {
		tpls, err := st.ListTemplates()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, tpls)
	})
	r.Post("/templates", func(w http.ResponseWriter, req *http.Request) {
		var t model.ArticleTemplate
		if err := json.NewDecoder(req.Body).Decode(&t); err != nil {
			writeErr(w, 400, err)
			return
		}
		if strings.TrimSpace(t.Name) == "" || strings.TrimSpace(t.Prompt) == "" {
			writeErr(w, 400, fmt.Errorf("name / prompt 必填"))
			return
		}
		if err := st.AddTemplate(&t); err != nil {
			writeErr(w, 409, err) // 重名 UNIQUE 冲突
			return
		}
		writeJSON(w, 201, t)
	})
	r.Patch("/templates/{id}", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		var t model.ArticleTemplate
		if err := json.NewDecoder(req.Body).Decode(&t); err != nil {
			writeErr(w, 400, err)
			return
		}
		t.ID = id
		if strings.TrimSpace(t.Name) == "" || strings.TrimSpace(t.Prompt) == "" {
			writeErr(w, 400, fmt.Errorf("name / prompt 必填"))
			return
		}
		if err := st.UpdateTemplate(&t); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(204)
	})
	r.Delete("/templates/{id}", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		if err := st.DeleteTemplate(id); err != nil {
			writeErr(w, 409, fmt.Errorf("删除失败(内置模板不可删除): %w", err))
			return
		}
		w.WriteHeader(204)
	})

	// --- 创作文章历史 ---
	r.Get("/articles", func(w http.ResponseWriter, req *http.Request) {
		creationID, _ := strconv.ParseInt(req.URL.Query().Get("creation_id"), 10, 64)
		if creationID <= 0 {
			writeErr(w, 400, fmt.Errorf("缺少 creation_id"))
			return
		}
		arts, err := st.ListArticles(creationID)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, arts)
	})
	r.Get("/articles/{id}", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		a, err := st.GetArticle(id)
		if err != nil {
			writeErr(w, 404, err)
			return
		}
		writeJSON(w, 200, a)
	})
	r.Patch("/articles/{id}", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		var a model.Article
		if err := json.NewDecoder(req.Body).Decode(&a); err != nil {
			writeErr(w, 400, err)
			return
		}
		a.ID = id
		if err := st.UpdateArticle(&a); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(204)
	})
	r.Delete("/articles/{id}", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		if err := st.DeleteArticle(id); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(204)
	})
}
