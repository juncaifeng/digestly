package store

import (
	"testing"
)

func TestGraphEntitiesAndRelations(t *testing.T) {
	st := openTest(t)

	id1, err := st.UpsertEntity("OpenAI", "组织")
	if err != nil {
		t.Fatal(err)
	}
	id2, err := st.UpsertEntity("GPT-5", "产品")
	if err != nil {
		t.Fatal(err)
	}
	// 归一 upsert: 大小写不同视为同一实体
	id1b, err := st.UpsertEntity("openai", "")
	if err != nil || id1b != id1 {
		t.Fatalf("normalized upsert: id=%d want %d err=%v", id1b, id1, err)
	}

	// 图谱三件套需要 item,先造一篇
	fid := addFeed(t, st, "f")
	it := addItem(t, st, fid, "g-graph")
	if err := st.LinkItemEntity(it, id1, "OpenAI", 2); err != nil {
		t.Fatal(err)
	}
	if err := st.AddRelation(id1, "发布了", id2, it, 0.9); err != nil {
		t.Fatal(err)
	}
	if err := st.AddRelation(id1, "发布了", id2, it, 0.9); err != nil {
		t.Fatal(err) // 幂等
	}

	es, err := st.ListEntities("open", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 1 || es[0].Name != "OpenAI" || es[0].ItemCount != 1 {
		t.Fatalf("entities = %+v", es)
	}
	rs, err := st.EntityRelations(id1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 || rs[0].Subject != "OpenAI" || rs[0].Predicate != "发布了" || rs[0].Object != "GPT-5" {
		t.Fatalf("relations = %+v", rs)
	}
}

func TestSimilarItemsAndCosine(t *testing.T) {
	if Cosine([]float32{1, 0}, []float32{1, 0}) != 1 {
		t.Fatal("cosine identical should be 1")
	}
	if Cosine([]float32{1, 0}, []float32{0, 1}) != 0 {
		t.Fatal("cosine orthogonal should be 0")
	}
	if Cosine([]float32{1}, []float32{1, 2}) != 0 {
		t.Fatal("dim mismatch should be 0")
	}

	st := openTest(t)
	fid := addFeed(t, st, "f")
	a := addItem(t, st, fid, "g-a")
	b := addItem(t, st, fid, "g-b")
	c := addItem(t, st, fid, "g-c")
	if err := st.SetItemEmbedding(a, "m", []float32{1, 0}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetItemEmbedding(b, "m", []float32{0.9, 0.1}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetItemEmbedding(c, "m", []float32{0, 1}); err != nil {
		t.Fatal(err)
	}
	sims, err := st.SimilarItems("m", []float32{1, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(sims) != 2 || sims[0].Item.ID != a || sims[1].Item.ID != b {
		t.Fatalf("sims = %+v", sims)
	}
	m, v, ok, err := st.GetItemEmbedding(a)
	if err != nil || !ok || m != "m" || v[0] != 1 {
		t.Fatalf("get embedding: %v %v %v %v", m, v, ok, err)
	}
	if _, _, ok, _ := st.GetItemEmbedding(9999); ok {
		t.Fatal("missing embedding should be ok=false")
	}
}
