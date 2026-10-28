package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/juncaifeng/digestly/core/llm"
	"github.com/juncaifeng/digestly/core/store"
)

// registerGraphRoutes 实体图谱查询 + 文章相似检索。
func registerGraphRoutes(r chi.Router, st *store.Store) {
	// 实体列表(按提及文章数降序)
	r.Get("/entities", func(w http.ResponseWriter, req *http.Request) {
		limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
		es, err := st.ListEntities(req.URL.Query().Get("q"), limit)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, es)
	})

	// 实体的关系(作为主语或宾语)
	r.Get("/entities/{id}/relations", func(w http.ResponseWriter, req *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		rs, err := st.EntityRelations(id)
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, rs)
	})

	// 相似文章: 传 item_id(用其向量)或 text(现算向量)
	r.Post("/similarity", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			ItemID int64  `json:"item_id"`
			Text   string `json:"text"`
			Limit  int    `json:"limit"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeErr(w, 400, err)
			return
		}
		limit := body.Limit
		if limit <= 0 {
			limit = 10
		}
		var modelName string
		var vec []float32
		switch {
		case body.ItemID > 0:
			m, v, ok, err := st.GetItemEmbedding(body.ItemID)
			if err != nil {
				writeErr(w, 500, err)
				return
			}
			if !ok {
				writeErr(w, 404, fmt.Errorf("该文章还没有向量(配置向量模型后等待标注管线处理)"))
				return
			}
			modelName, vec = m, v
		case body.Text != "":
			cfg, err := st.DefaultConfigOfKind("embedding")
			if err != nil {
				writeErr(w, 422, fmt.Errorf("还没有默认向量模型配置,请先在「设置」页添加 embedding 配置"))
				return
			}
			vecs, err := llm.Embed(req.Context(), cfg, []string{body.Text})
			if err != nil {
				writeErr(w, 502, err)
				return
			}
			modelName, vec = cfg.Model, vecs[0]
		default:
			writeErr(w, 400, fmt.Errorf("需要 item_id 或 text"))
			return
		}
		sims, err := st.SimilarItems(modelName, vec, limit+1) // 多取一条用于排除自身
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		out := sims[:0]
		for _, sm := range sims {
			if sm.Item.ID == body.ItemID {
				continue
			}
			out = append(out, sm)
			if len(out) >= limit {
				break
			}
		}
		writeJSON(w, 200, out)
	})
}
