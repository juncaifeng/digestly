package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/juncaifeng/digestly/core/compose"
	"github.com/juncaifeng/digestly/core/model"
	"github.com/juncaifeng/digestly/core/store"
)

// 创作单元: 一等公民,素材源可混合专栏与单个订阅源。
func registerCreationRoutes(r chi.Router, st *store.Store) {
	r.Get("/creations", func(w http.ResponseWriter, _ *http.Request) {
		cs, err := st.ListCreations()
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, cs)
	})
	r.Post("/creations", func(w http.ResponseWriter, req *http.Request) {
		var c model.Creation
		if err := json.NewDecoder(req.Body).Decode(&c); err != nil {
			writeErr(w, 400, err)
			return
		}
		if strings.TrimSpace(c.Name) == "" {
			writeErr(w, 400, fmt.Errorf("缺少 name"))
			return
		}
		if err := st.AddCreation(&c); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 201, c)
	})
	r.Patch("/creations/{id}", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		var c model.Creation
		if err := json.NewDecoder(req.Body).Decode(&c); err != nil {
			writeErr(w, 400, err)
			return
		}
		c.ID = id
		if strings.TrimSpace(c.Name) == "" {
			writeErr(w, 400, fmt.Errorf("缺少 name"))
			return
		}
		if err := st.UpdateCreation(&c); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(204)
	})
	r.Delete("/creations/{id}", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		if err := st.DeleteCreation(id); err != nil {
			writeErr(w, 500, err)
			return
		}
		w.WriteHeader(204)
	})
	// 预览: 解析创作的模板 + 素材 + 完整 prompt,不调用 LLM、无副作用。
	// 供外部 agent 取用: 可直接用 prompt,或按 item_ids 分页细读素材(GET /items/{id})。
	r.Post("/creations/{id}/plan", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		var body struct {
			TemplateID int64 `json:"template_id"`
			Force      bool  `json:"force"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		p, err := compose.BuildPlan(st, id, body.TemplateID, body.Force)
		if err != nil {
			writeErr(w, 422, err)
			return
		}
		writeJSON(w, 200, p)
	})
	// 生成: 展开素材源 -> 取最新未读 -> LLM -> 落库 -> 素材标记已读
	// template_id 可省,缺省用创作关联的默认模板; force = 不限未读,取最新素材
	r.Post("/creations/{id}/generate", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		var body struct {
			TemplateID int64 `json:"template_id"`
			Force      bool  `json:"force"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		a, err := compose.Generate(req.Context(), st, id, body.TemplateID, body.Force)
		if err != nil {
			writeErr(w, 422, err)
			return
		}
		writeJSON(w, 201, a)
	})
}
